"""将图片 OCR 结果转换为可安全进入 Markdown alt 的完整描述。"""

import re
from collections.abc import Callable
from pathlib import Path


ImageRecognizer = Callable[[Path], str]


def normalize_image_alt(text: str, fallback: str) -> str:
    """压平空白并替换会结束 Markdown alt 的方括号，不截断 OCR 正文。"""
    normalized = re.sub(r"\s+", " ", text or "").strip()
    normalized = normalized.replace("[", "（").replace("]", "）")
    return normalized or fallback


def build_image_markdown(image_path: Path, recognizer: ImageRecognizer) -> str:
    """识别图片并返回 Markdown；单图 OCR 失败时以文件名作为描述。"""
    try:
        text = recognizer(image_path)
    except Exception:  # noqa: BLE001 - 单图识别失败必须降级，不能中断整份文档
        text = ""
    alt = normalize_image_alt(text, image_path.name)
    return f"![{alt}]({image_path.as_posix()})"
