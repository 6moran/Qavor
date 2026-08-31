import io
import importlib
import json
import unittest
from pathlib import Path

from pkg.documentparser.python.protocol import PROTOCOL_VERSION, serve_stdio


def parse_request(request_id: str = "r1") -> str:
    return json.dumps(
        {
            "type": "parse",
            "version": PROTOCOL_VERSION,
            "request_id": request_id,
            "input_path": "a.pdf",
            "filename": "a.pdf",
            "ocr_engine": "rapidocr",
        }
    )


class ServeStdioTests(unittest.TestCase):
    def test_serve_stdio_emits_ready_and_correlated_result(self) -> None:
        stdin = io.StringIO(parse_request() + "\n")
        stdout = io.StringIO()
        stderr = io.StringIO()

        serve_stdio(lambda request: {"markdown": "hello", "metadata": {}}, stdin, stdout, stderr)

        messages = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(messages[0]["type"], "ready")
        self.assertEqual(messages[0]["version"], PROTOCOL_VERSION)
        self.assertEqual(messages[1]["request_id"], "r1")
        self.assertTrue(messages[1]["ok"])
        self.assertEqual(messages[1]["result"]["markdown"], "hello")

    def test_serve_stdio_returns_safe_parser_error(self) -> None:
        stdin = io.StringIO(parse_request() + "\n")
        stdout = io.StringIO()
        stderr = io.StringIO()

        def fail_parser(_request: dict[str, object]) -> dict[str, object]:
            raise RuntimeError("credential=secret")

        serve_stdio(fail_parser, stdin, stdout, stderr)

        result = [json.loads(line) for line in stdout.getvalue().splitlines()][1]
        self.assertFalse(result["ok"])
        self.assertEqual(result["request_id"], "r1")
        self.assertEqual(result["error"]["code"], "PARSER_FAILED")
        self.assertNotIn("secret", stdout.getvalue())
        self.assertIn("credential=secret", stderr.getvalue())

    def test_serve_stdio_preserves_safe_parser_error_code(self) -> None:
        stdin = io.StringIO(parse_request() + "\n")
        stdout = io.StringIO()

        class ExpectedParserError(Exception):
            code = "PARSER_FILE_NOT_FOUND"
            message = "输入文件不存在"

        def fail_parser(_request: dict[str, object]) -> dict[str, object]:
            raise ExpectedParserError("untrusted detail")

        serve_stdio(fail_parser, stdin, stdout, io.StringIO())

        result = [json.loads(line) for line in stdout.getvalue().splitlines()][1]
        self.assertEqual(result["error"]["code"], "PARSER_FILE_NOT_FOUND")
        self.assertEqual(result["error"]["message"], "document parsing failed")

    def test_serve_stdio_reports_malformed_json(self) -> None:
        stdout = io.StringIO()
        stderr = io.StringIO()

        serve_stdio(lambda request: {}, io.StringIO("{bad json}\n"), stdout, stderr)

        result = [json.loads(line) for line in stdout.getvalue().splitlines()][1]
        self.assertFalse(result["ok"])
        self.assertEqual(result["error"]["code"], "PARSER_PROTOCOL_ERROR")
        self.assertNotIn("request_id", result)
        self.assertTrue(stderr.getvalue())

    def test_serve_stdio_correlates_validation_errors(self) -> None:
        invalid = json.dumps(
            {
                "type": "parse",
                "version": PROTOCOL_VERSION,
                "request_id": "bad-request",
                "filename": "a.pdf",
                "ocr_engine": "rapidocr",
            }
        )
        stdout = io.StringIO()

        serve_stdio(lambda request: {}, io.StringIO(invalid + "\n"), stdout, io.StringIO())

        result = [json.loads(line) for line in stdout.getvalue().splitlines()][1]
        self.assertFalse(result["ok"])
        self.assertEqual(result["request_id"], "bad-request")
        self.assertEqual(result["error"]["code"], "PARSER_PROTOCOL_ERROR")

    def test_serve_stdio_continues_after_request_error(self) -> None:
        stdin = io.StringIO("{bad json}\n" + parse_request("r2") + "\n")
        stdout = io.StringIO()

        serve_stdio(lambda request: {"markdown": request["filename"]}, stdin, stdout, io.StringIO())

        messages = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual(len(messages), 3)
        self.assertFalse(messages[1]["ok"])
        self.assertTrue(messages[2]["ok"])
        self.assertEqual(messages[2]["request_id"], "r2")


class ParsePathTests(unittest.TestCase):
    def test_parse_path_validates_a_missing_file_without_loading_backends(self) -> None:
        parser = importlib.import_module("pkg.documentparser.python.parse_document")

        with self.assertRaises(parser.ParserError) as raised:
            parser.parse_path(Path("does-not-exist.pdf"))

        self.assertEqual(raised.exception.code, "PARSER_FILE_NOT_FOUND")


if __name__ == "__main__":
    unittest.main()
