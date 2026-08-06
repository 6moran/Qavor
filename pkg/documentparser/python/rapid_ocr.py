"""
RapidOCR 解析器

使用 RapidOCR (PP-OCRv5, ONNX Runtime, 中英文) 识别图片文本;
PDF 使用 PyMuPDF 逐页渲染为图片后识别,流式处理避免内存峰值。
模型单例延迟加载,进程内只加载一次。
"""

import os
import tempfile
import threading
from pathlib import Path

import fitz
from PIL import Image
from rapidocr import EngineType, LangDet, LangRec, ModelType, OCRVersion, RapidOCR

_ocr: RapidOCR | None = None
_ocr_lock = threading.Lock()

# 允许识别的图片扩展名
IMAGE_EXTENSIONS = (".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".tif")


def _get_ocr() -> RapidOCR:
    """获取 RapidOCR 单例(PP-OCRv5, 中英文, ONNX Runtime, Mobile 模型)。"""
    global _ocr
    if _ocr is None:
        with _ocr_lock:
            if _ocr is None:
                _ocr = RapidOCR(
                    params={
                        "Det.engine_type": EngineType.ONNXRUNTIME,
                        "Det.lang_type": LangDet.CH,
                        "Det.model_type": ModelType.MOBILE,
                        "Det.ocr_version": OCRVersion.PPOCRV5,
                        "Det.box_thresh": 0.3,
                        "Cls.engine_type": EngineType.ONNXRUNTIME,
                        "Rec.engine_type": EngineType.ONNXRUNTIME,
                        "Rec.lang_type": LangRec.CH,
                        "Rec.model_type": ModelType.MOBILE,
                        "Rec.ocr_version": OCRVersion.PPOCRV5,
                    }
                )
    return _ocr


def _run_ocr(ocr: RapidOCR, image_path: str) -> str:
    """执行 OCR 并返回按行拼接的文本,未识别到内容时返回空串。"""
    result = ocr(image_path)
    if result and result.txts:
        return "\n".join(result.txts)
    return ""


def ocr_image(image: str | Path | Image.Image) -> str:
    """识别单张图片,支持文件路径或 PIL Image 对象。"""
    ocr = _get_ocr()
    if isinstance(image, Image.Image):
        with tempfile.NamedTemporaryFile(mode="wb", suffix=".png", delete=False) as tmp:
            image.save(tmp.name)
            temp_path = tmp.name
        try:
            return _run_ocr(ocr, temp_path)
        finally:
            if os.path.exists(temp_path):
                os.remove(temp_path)
    return _run_ocr(ocr, str(image))


def ocr_pdf(pdf_path: str | Path, zoom: float = 2.0) -> tuple[str, list[dict]]:
    """逐页渲染 PDF 为图片并 OCR,返回 (markdown, pages)。

    markdown 中每页以 <!-- page:N --> 注释分隔;pages 为 [{number, text}]。
    """
    pdf = fitz.open(str(pdf_path))
    pages: list[dict] = []
    texts: list[str] = []
    matrix = fitz.Matrix(zoom, zoom)
    try:
        for number in range(pdf.page_count):
            pix = pdf[number].get_pixmap(matrix=matrix, alpha=False)
            img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
            text = ocr_image(img)
            pages.append({"number": number + 1, "text": text})
            if text:
                texts.append(f"<!-- page:{number + 1} -->\n{text}")
    finally:
        pdf.close()
    return "\n\n".join(texts).strip(), pages
