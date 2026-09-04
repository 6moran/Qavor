"""Real-format parser acceptance tests.

These deliberately exercise the installed Docling and RapidOCR backends.  They
are opt-in because the backends can download models on first use; lightweight
protocol and routing tests remain the normal default suite.
"""

from __future__ import annotations

import importlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Any

from pkg.documentparser.python.parse_document import ParserError, parse_path


TESTDATA = Path(__file__).with_name("testdata")
EXPECTED = json.loads((TESTDATA / "expected.json").read_text(encoding="utf-8"))


def real_parser_dependencies_available() -> tuple[bool, str]:
    """Check imports without constructing a converter or loading OCR models."""
    required = ("docling.document_converter", "rapidocr", "fitz", "PIL")
    for module in required:
        try:
            importlib.import_module(module)
        except Exception as exc:  # noqa: BLE001 - this must become an explicit skip
            return False, f"real parser dependency {module!r} is unavailable: {exc}"
    return True, ""


REAL_PARSER_TESTS_ENABLED = os.getenv("QAVOR_REAL_PARSER_TESTS") == "1"
if REAL_PARSER_TESTS_ENABLED:
    REAL_DEPS_AVAILABLE, REAL_DEPS_SKIP_REASON = real_parser_dependencies_available()
    if not REAL_DEPS_AVAILABLE:
        raise RuntimeError(
            "QAVOR_REAL_PARSER_TESTS=1 requires real parser dependencies: "
            f"{REAL_DEPS_SKIP_REASON}"
        )


@unittest.skipUnless(
    REAL_PARSER_TESTS_ENABLED,
    "set QAVOR_REAL_PARSER_TESTS=1 to run real parser integration tests",
)
class RealParserIntegrationTests(unittest.TestCase):
    def assert_markers(self, filename: str, markdown: str) -> None:
        for marker in EXPECTED[filename]:
            # OCR engines can omit layout-only spaces around CJK text. Preserve
            # the marker words and numbers while accepting that representation.
            self.assertIn(
                re.sub(r"\s+", "", marker),
                re.sub(r"\s+", "", markdown),
                f"{filename} is missing {marker!r}",
            )

    def test_digital_pdf_uses_docling_without_fallback(self) -> None:
        result = parse_path(TESTDATA / "digital.pdf")

        self.assertEqual(result.metadata.get("parser"), "docling")
        self.assertFalse(result.metadata.get("fallback"))
        self.assert_markers("digital.pdf", result.markdown)

    def test_scanned_pdf_falls_back_to_ocr_with_metadata(self) -> None:
        result = parse_path(TESTDATA / "scanned.pdf")

        self.assertTrue(result.metadata.get("fallback"))
        self.assertIn(result.metadata.get("parser"), {"rapidocr", "api_ocr"})
        self.assertIn(result.metadata.get("fallback_reason"), {"docling_empty", "docling_failed"})
        self.assert_markers("scanned.pdf", result.markdown)

    def test_office_documents_and_image_keep_expected_content(self) -> None:
        # Docling exports embedded pictures next to its input. Use a disposable
        # copy so an integration run never changes committed fixtures.
        with tempfile.TemporaryDirectory() as directory:
            docx_path = Path(directory) / "with-image.docx"
            shutil.copy2(TESTDATA / "with-image.docx", docx_path)
            docx = parse_path(docx_path)
            self.assertEqual(docx.metadata.get("parser"), "docling")
            self.assert_markers("with-image.docx", docx.markdown)
            alt_texts = re.findall(r"!\[([^\]]*)\]\(", docx.markdown)
            self.assertTrue(
                any(
                    "图片文字271828" in re.sub(r"\s+", "", alt)
                    for alt in alt_texts
                ),
                f"DOCX image OCR is not present in Markdown alt text: {docx.markdown!r}",
            )

        for filename in ("slides.pptx", "table.xlsx", "text-image.png"):
            with self.subTest(filename=filename):
                result = parse_path(TESTDATA / filename)
                self.assert_markers(filename, result.markdown)

    def test_corrupted_pdf_returns_safe_parser_error(self) -> None:
        with self.assertRaises(ParserError) as raised:
            parse_path(TESTDATA / "corrupted.pdf")

        self.assertEqual(raised.exception.code, "PARSER_FAILED")
        self.assertEqual(raised.exception.message, "文档解析失败")

if __name__ == "__main__":
    unittest.main()
