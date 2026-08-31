import io
import unittest
from contextlib import redirect_stderr
from pathlib import Path
from unittest.mock import patch

from pkg.documentparser.python import parse_document


class ParsePdfTests(unittest.TestCase):
    def test_pdf_prefers_docling_when_visible_content_exists(self) -> None:
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(
                 parse_document,
                 "_convert_with_docling",
                 return_value="# 数字版标题\n正文 2026",
             ) as docling, \
             patch.object(parse_document, "ocr_pdf") as ocr:
            result = parse_document.parse_path(Path("digital.pdf"))

        self.assertEqual(
            result.metadata,
            {"file_type": ".pdf", "parser": "docling", "fallback": False},
        )
        self.assertEqual(result.markdown, "# 数字版标题\n正文 2026")
        docling.assert_called_once()
        ocr.assert_not_called()

    def test_pdf_falls_back_when_docling_is_empty(self) -> None:
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(parse_document, "_convert_with_docling", return_value="  #  \n"), \
             patch.object(
                 parse_document,
                 "ocr_pdf",
                 return_value=("扫描正文", [{"number": 1, "text": "扫描正文"}]),
             ) as ocr:
            result = parse_document.parse_path(Path("scan.pdf"))

        self.assertTrue(result.metadata["fallback"])
        self.assertEqual(result.metadata["fallback_reason"], "docling_empty")
        self.assertEqual(result.metadata["parser"], "rapidocr")
        self.assertEqual(result.pages, [{"number": 1, "text": "扫描正文"}])
        ocr.assert_called_once_with(Path("scan.pdf"))

    def test_pdf_falls_back_once_when_docling_raises(self) -> None:
        stderr = io.StringIO()
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(parse_document, "_convert_with_docling", side_effect=RuntimeError("Docling offline")), \
             patch.object(
                 parse_document,
                 "ocr_pdf",
                 return_value=("OCR 正文", [{"number": 1, "text": "OCR 正文"}]),
             ) as ocr, \
             redirect_stderr(stderr):
            result = parse_document.parse_path(Path("broken.pdf"))

        self.assertEqual(
            result.metadata,
            {
                "file_type": ".pdf",
                "parser": "rapidocr",
                "fallback": True,
                "fallback_reason": "docling_failed",
            },
        )
        self.assertEqual(result.pages, [{"number": 1, "text": "OCR 正文"}])
        self.assertIn("Docling offline", stderr.getvalue())
        ocr.assert_called_once_with(Path("broken.pdf"))


if __name__ == "__main__":
    unittest.main()
