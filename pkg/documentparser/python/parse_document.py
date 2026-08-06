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

from docling.datamodel.base_models import InputFormat
from docling.document_converter import DocumentConverter

from rapid_ocr import ocr_image, ocr_pdf


@dataclass
class ParseResult:
    """解析结果,与 Go 侧 ingestion.ParseResult 字段一一对应。"""

    markdown: str
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
    for pic in doc.pictures:
        uri = str(pic.image.uri) if hasattr(pic, "image") and hasattr(pic.image, "uri") else ""
        if uri.startswith("data:"):
            try:
                image_data, mime_type = _parse_data_uri(uri)
                ext = mime_type.split("/")[-1]
                name = f"img_{int(time.time() * 1_000_000)}.{ext}"
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
    args = parser.parse_args()
    path = Path(args.input)
    if not path.is_file():
        fail("PARSER_FILE_NOT_FOUND", "输入文件不存在")
    suffix = path.suffix.lower()
    result = ParseResult(metadata={"file_type": suffix})
    try:
        if suffix in (".docx", ".pptx", ".xlsx"):
            result.markdown = _convert_with_docling(path, result)
            result.metadata["parser"] = "docling"
        elif suffix == ".pdf":
            result.markdown, result.pages = ocr_pdf(path)
            result.metadata["parser"] = "rapidocr"
        elif suffix in (".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif"):
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
