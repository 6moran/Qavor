"""
通用 OCR API 解析器

通过 HTTP 接口调用外部 OCR 服务识别图片文本。约定：
- 请求：POST multipart/form-data，图片以文件字段 `image` 提交；
  若配置了 API Key，附带请求头 `Authorization: Bearer <key>`；
- 响应：优先解析 JSON 的 `text` 字段；否则将原始响应体作为纯文本返回。

PDF 使用 PyMuPDF 逐页渲染为图片后逐页调用，与 rapid_ocr 的流式处理一致，
避免整份 PDF 一次性上传带来的内存/超时风险。
"""

import io
import sys
from pathlib import Path
from typing import Any

import fitz
import requests
from PIL import Image

# 单页识别超时（秒），覆盖上传与识别的总耗时
REQUEST_TIMEOUT_SECONDS = 60


def _extract_text(response: requests.Response) -> str:
    """从 OCR API 响应中提取文本：优先 JSON 的 text 字段，否则返回原始文本。"""
    content_type = response.headers.get("Content-Type", "")
    if "application/json" in content_type or "text/json" in content_type:
        try:
            payload: Any = response.json()
        except ValueError:
            return response.text
        if isinstance(payload, dict):
            text = payload.get("text")
            if isinstance(text, str) and text.strip():
                return text
            # 兼容嵌套结构 {"data": {"text": ...}}
            data = payload.get("data")
            if isinstance(data, dict):
                text = data.get("text")
                if isinstance(text, str) and text.strip():
                    return text
        return response.text
    return response.text


def ocr_image_api(image_path: str, base_url: str, api_key: str = "", model: str = "") -> str:
    """识别单张图片，返回按行拼接的文本。model 以表单字段提交，可为空。"""
    headers = {}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    data = {"model": model} if model else None
    with open(image_path, "rb") as f:
        response = requests.post(
            base_url,
            files={"image": (Path(image_path).name, f)},
            data=data,
            headers=headers,
            timeout=REQUEST_TIMEOUT_SECONDS,
        )
    if response.status_code != 200:
        raise RuntimeError(
            f"OCR API 调用失败: HTTP {response.status_code} - {response.text[:200]}"
        )
    return _extract_text(response)


def ocr_image_bytes_api(image_bytes: bytes, filename: str, base_url: str, api_key: str = "", model: str = "") -> str:
    """识别内存中的图片字节（供 PDF 渲染页使用）。model 以表单字段提交，可为空。"""
    headers = {}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    data = {"model": model} if model else None
    response = requests.post(
        base_url,
        files={"image": (filename, image_bytes)},
        data=data,
        headers=headers,
        timeout=REQUEST_TIMEOUT_SECONDS,
    )
    if response.status_code != 200:
        raise RuntimeError(
            f"OCR API 调用失败: HTTP {response.status_code} - {response.text[:200]}"
        )
    return _extract_text(response)


def ocr_pdf_api(pdf_path: str, base_url: str, api_key: str = "", model: str = "", zoom: float = 2.0) -> tuple[str, list[dict]]:
    """逐页渲染 PDF 并调用 API 识别，返回 (markdown, pages)。

    markdown 中每页以 <!-- page:N --> 注释分隔；pages 为 [{number, text}]。
    """
    pdf = fitz.open(str(pdf_path))
    pages: list[dict] = []
    texts: list[str] = []
    matrix = fitz.Matrix(zoom, zoom)
    try:
        for number in range(pdf.page_count):
            try:
                pix = pdf[number].get_pixmap(matrix=matrix, alpha=False)
                img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
                buf = io.BytesIO()
                img.save(buf, format="PNG")
                text = ocr_image_bytes_api(buf.getvalue(), f"page_{number + 1}.png", base_url, api_key, model)
            except Exception as exc:  # noqa: BLE001
                # 单页失败降级为空文本，不中断整个文档解析
                print(f"第 {number + 1} 页 OCR 失败: {exc}", file=sys.stderr)
                text = ""
            pages.append({"number": number + 1, "text": text})
            if text:
                texts.append(f"<!-- page:{number + 1} -->\n{text}")
    finally:
        pdf.close()
    return "\n\n".join(texts).strip(), pages
