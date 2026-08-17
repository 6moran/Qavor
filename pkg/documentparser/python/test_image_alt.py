import unittest
from pathlib import Path
from unittest.mock import patch

from image_alt import build_image_markdown, normalize_image_alt
from parse_document import _build_picture_recognizer


class NormalizeImageAltTest(unittest.TestCase):
    def test_keeps_all_ocr_text_and_flattens_whitespace(self) -> None:
        text = "第一行\n第二行\t  第三行"

        self.assertEqual(normalize_image_alt(text, "img.png"), "第一行 第二行 第三行")

    def test_replaces_markdown_brackets_without_truncating_text(self) -> None:
        text = "销量[万元] 与完成率]"

        self.assertEqual(normalize_image_alt(text, "img.png"), "销量（万元） 与完成率）")

    def test_uses_filename_when_ocr_returns_empty_text(self) -> None:
        self.assertEqual(normalize_image_alt(" \n ", "chart.png"), "chart.png")


class BuildImageMarkdownTest(unittest.TestCase):
    def test_builds_markdown_with_complete_ocr_text(self) -> None:
        path = Path("C:/tmp/chart.png")

        result = build_image_markdown(path, lambda _: "一季度 120 万\n二季度 165 万")

        self.assertEqual(
            result,
            "![一季度 120 万 二季度 165 万](C:/tmp/chart.png)",
        )

    def test_falls_back_to_filename_when_ocr_raises(self) -> None:
        def failing_recognizer(_: Path) -> str:
            raise RuntimeError("ocr unavailable")

        result = build_image_markdown(Path("C:/tmp/chart.png"), failing_recognizer)

        self.assertEqual(result, "![chart.png](C:/tmp/chart.png)")


class PictureRecognizerFactoryTest(unittest.TestCase):
    @patch("parse_document.ocr_image", return_value="本地识别结果")
    def test_rapidocr_recognizer_uses_local_engine(self, mocked_ocr) -> None:
        recognizer = _build_picture_recognizer("rapidocr", "", "", "")

        self.assertEqual(recognizer(Path("chart.png")), "本地识别结果")
        mocked_ocr.assert_called_once_with(Path("chart.png"))

    @patch("parse_document.ocr_image_api", return_value="接口识别结果")
    def test_api_recognizer_forwards_configuration(self, mocked_ocr) -> None:
        recognizer = _build_picture_recognizer("api", "https://ocr.test", "secret", "vision")

        self.assertEqual(recognizer(Path("chart.png")), "接口识别结果")
        mocked_ocr.assert_called_once_with(
            Path("chart.png"), "https://ocr.test", "secret", "vision"
        )


if __name__ == "__main__":
    unittest.main()
