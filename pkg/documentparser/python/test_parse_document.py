import io
import tempfile
import unittest
from contextlib import redirect_stderr
from pathlib import Path
from unittest.mock import patch

from pkg.documentparser.python import parse_document


class ParsePdfTests(unittest.TestCase):
    def test_pdf_with_text_layer_uses_native_extraction(self) -> None:
        markdown, pages = parse_document._extract_pdf_text(
            Path(__file__).with_name("testdata") / "digital.pdf"
        )

        self.assertIn("Qavor数字PDF", markdown.replace("\xa0", ""))
        self.assertEqual(len(pages), 1)

    def test_missing_path_with_windows_surrogate_returns_parser_error(self) -> None:
        stderr_bytes = io.BytesIO()
        stderr = io.TextIOWrapper(stderr_bytes, encoding="utf-8", errors="strict")
        self.addCleanup(stderr.detach)

        with patch.object(Path, "is_file", return_value=False), redirect_stderr(stderr):
            with self.assertRaises(parse_document.ParserError) as raised:
                parse_document.parse_path(Path("missing-\udca9.pdf"))

        self.assertEqual(raised.exception.code, "PARSER_FILE_NOT_FOUND")

    def test_pdf_docling_picture_placeholder_uses_one_whole_pdf_fallback(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "image-only.pdf"
            path.touch()
            with patch.object(parse_document, "_extract_pdf_text", return_value=("", [])), \
                 patch.object(parse_document, "ocr_image", return_value="图片文字") as image_ocr, \
                 patch.object(
                     parse_document,
                     "ocr_pdf",
                     return_value=("整份 OCR 正文", [{"number": 1, "text": "整份 OCR 正文"}]),
                 ) as pdf_ocr:
                result = parse_document.parse_path(path)

        self.assertEqual(result.markdown, "整份 OCR 正文")
        self.assertEqual(result.metadata["fallback_reason"], "text_layer_empty")
        self.assertEqual(result.pages, [{"number": 1, "text": "整份 OCR 正文"}])
        image_ocr.assert_not_called()
        pdf_ocr.assert_called_once_with(path)

    def test_pdf_prefers_docling_when_visible_content_exists(self) -> None:
        stderr = io.StringIO()
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(
                 parse_document,
                 "_extract_pdf_text",
                 return_value=("# 数字版标题\n正文 2026", [{"number": 1, "text": "正文 2026"}]),
             ), \
             patch.object(parse_document, "ocr_pdf") as ocr, \
             redirect_stderr(stderr):
            result = parse_document.parse_path(Path("digital.pdf"))

        self.assertEqual(
            result.metadata,
            {"file_type": ".pdf", "parser": "pymupdf", "fallback": False},
        )
        self.assertEqual(result.markdown, "# 数字版标题\n正文 2026")
        self.assertIn("数字版文字层提取", stderr.getvalue())
        ocr.assert_not_called()

    def test_pdf_falls_back_when_docling_is_empty(self) -> None:
        stderr = io.StringIO()
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(parse_document, "_extract_pdf_text", return_value=("  #  \n", [])), \
             patch.object(
                 parse_document,
                 "ocr_pdf",
                 return_value=("扫描正文", [{"number": 1, "text": "扫描正文"}]),
             ) as ocr, \
             redirect_stderr(stderr):
            result = parse_document.parse_path(Path("scan.pdf"))

        self.assertTrue(result.metadata["fallback"])
        self.assertEqual(result.metadata["fallback_reason"], "text_layer_empty")
        self.assertEqual(result.metadata["parser"], "rapidocr")
        self.assertEqual(result.pages, [{"number": 1, "text": "扫描正文"}])
        self.assertIn("扫描版 OCR", stderr.getvalue())
        ocr.assert_called_once_with(Path("scan.pdf"))

    def test_pdf_falls_back_once_when_docling_raises(self) -> None:
        stderr = io.StringIO()
        with patch.object(Path, "is_file", return_value=True), \
             patch.object(parse_document, "_extract_pdf_text", return_value=("", [])), \
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
                "fallback_reason": "text_layer_empty",
            },
        )
        self.assertEqual(result.pages, [{"number": 1, "text": "OCR 正文"}])
        self.assertIn("扫描版 OCR", stderr.getvalue())
        ocr.assert_called_once_with(Path("broken.pdf"))


if __name__ == "__main__":
    unittest.main()
