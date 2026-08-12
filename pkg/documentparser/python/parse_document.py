#!/usr/bin/env python3
"""
文档解析模块

将支持的文档转换为 Markdown 并输出 JSON:
- .docx/.pptx/.xlsx 通过 Docling 转换,文档内图片导出为临时文件并以路径引用
- .pdf 逐页渲染为图片后使用 RapidOCR 识别
- .jpg/.jpeg/.png/.bmp/.tiff/.tif 使用 RapidOCR 直接识别

输出格式:JSON 对象,与 Go 侧 ingestion.ParseResult 字段一一对应:
- markdown: 转换后的 Markdown 文本
- picture_paths: 提取的图片临时文件路径列表(绝对路径,正斜杠)
- pages: 页面信息列表(仅 PDF)
- metadata: 文件元数据(文件类型、解析器)

错误协议:{"error_code": "...", "error_message": "..."}
"""

import argparse
import base64
import json
import re
import sys
import threading
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any

# Windows 下管道输出默认使用 GBK 编码,Go 侧按 UTF-8 解析,必须显式统一编码
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

from docling.datamodel.base_models import InputFormat
from docling.document_converter import DocumentConverter

from api_ocr import ocr_image_api, ocr_pdf_api
from rapid_ocr import IMAGE_EXTENSIONS, ocr_image, ocr_pdf


@dataclass
class ParseResult:
    """解析结果,与 Go 侧 ingestion.ParseResult 字段一一对应。"""

    markdown: str = ""
    picture_paths: list[str] = field(default_factory=list)
    pages: list[dict[str, Any]] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)


def fail(code: str, message: str) -> None:
    """输出错误信息并退出程序。"""
    print(json.dumps({"error_code": code, "error_message": message}, ensure_ascii=False))
    raise SystemExit(2)


_docling_converter: DocumentConverter | None = None
_docling_lock = threading.Lock()


def _get_docling_converter() -> DocumentConverter:
    """获取 Docling 转换器单例(进程内只加载一次)。"""
    global _docling_converter
    if _docling_converter is None:
        with _docling_lock:
            if _docling_converter is None:
                _docling_converter = DocumentConverter(
                    format_options={
                        InputFormat.DOCX: None,
                        InputFormat.XLSX: None,
                        InputFormat.PPTX: None,
                    }
                )
    return _docling_converter


def _parse_data_uri(data_uri: str) -> tuple[bytes, str]:
    """解析 data URI,返回 (image_data, mime_type)。"""
    header, base64_data = data_uri.split(",", 1)
    mime_type = header.split(":")[1].split(";")[0]
    return base64.b64decode(base64_data), mime_type


def _convert_with_docling(file_path: Path, result: ParseResult) -> str:
    """使用 Docling 转换 docx/xlsx/pptx,图片导出到输入文件同级 images/ 目录。

    导出路径以绝对路径(正斜杠)写入 markdown 引用并加入 picture_paths,
    由 Go 侧上传 MinIO 后回填 URL。单张图片导出失败降级为文本占位,不中断解析。
    """
    converter = _get_docling_converter()
    converted = converter.convert(file_path)
    if converted.status.name != "SUCCESS":
        raise RuntimeError(f"Docling 转换失败: {converted.status}")

    doc = converted.document
    if not (hasattr(doc, "pictures") and doc.pictures):
        return doc.export_to_markdown()

    images_dir = file_path.parent / "images"
    images_dir.mkdir(exist_ok=True)
    replacements: list[str] = []
    for index, pic in enumerate(doc.pictures):
        uri = str(pic.image.uri) if hasattr(pic, "image") and hasattr(pic.image, "uri") else ""
        if uri.startswith("data:"):
            try:
                image_data, mime_type = _parse_data_uri(uri)
                ext = mime_type.split("/")[-1]
                name = f"img_{int(time.time() * 1_000_000)}_{index}.{ext}"
                image_path = images_dir / name
                image_path.write_bytes(image_data)
                posix = image_path.as_posix()
                result.picture_paths.append(posix)
                replacements.append(f"![{name}]({posix})")
            except Exception as exc:  # noqa: BLE001
                print(f"图片导出失败: {exc}", file=sys.stderr)
                replacements.append("[图片: 导出失败]")
        else:
            replacements.append("")

    markdown = doc.export_to_markdown()
    for replacement in replacements:
        # 使用 lambda 避免 replacement 中的反斜杠/分组被 re.sub 解释
        markdown = re.sub(r"<!--\s*image\s*-->", lambda _m: replacement, markdown, count=1)
    return markdown


def main() -> None:
    """解析命令行参数并执行文档解析,输出 JSON 结果。"""
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument(
        "--ocr-engine",
        choices=("rapidocr", "api"),
        default="rapidocr",
        help="图片/PDF 使用的 OCR 引擎: rapidocr(本地,默认) 或 api(通用 OCR API)",
    )
    parser.add_argument("--ocr-api-url", default="", help="通用 OCR API 接口地址")
    parser.add_argument("--ocr-api-key", default="", help="通用 OCR API 访问凭证")
    parser.add_argument("--ocr-api-model", default="", help="通用 OCR API 模型名称")
    args = parser.parse_args()
    path = Path(args.input)
    if not path.is_file():
        fail("PARSER_FILE_NOT_FOUND", "输入文件不存在")
    if args.ocr_engine == "api" and not args.ocr_api_url:
        fail("PARSER_OCR_CONFIG_MISSING", "未配置 OCR API 服务地址")
    suffix = path.suffix.lower()
    result = ParseResult(metadata={"file_type": suffix})
    try:
        if suffix in (".docx", ".pptx", ".xlsx"):
            result.markdown = _convert_with_docling(path, result)
            result.metadata["parser"] = "docling"
        elif suffix == ".pdf":
            if args.ocr_engine == "api":
                result.markdown, result.pages = ocr_pdf_api(path, args.ocr_api_url, args.ocr_api_key, args.ocr_api_model)
                result.metadata["parser"] = "api_ocr"
            else:
                result.markdown, result.pages = ocr_pdf(path)
                result.metadata["parser"] = "rapidocr"
        elif suffix in IMAGE_EXTENSIONS:
            if args.ocr_engine == "api":
                result.markdown = ocr_image_api(path, args.ocr_api_url, args.ocr_api_key, args.ocr_api_model)
                result.metadata["parser"] = "api_ocr"
            else:
                result.markdown = ocr_image(path)
                result.metadata["parser"] = "rapidocr"
        else:
            fail("PARSER_UNSUPPORTED_TYPE", f"不支持的文档类型: {suffix}")
    except SystemExit:
        raise
    except Exception as exc:
        print(f"parser failed: {exc}", file=sys.stderr)
        fail("PARSER_FAILED", "文档解析失败")
    print(json.dumps(asdict(result), ensure_ascii=False))


if __name__ == "__main__":
    main()
