"""Persistent JSONL protocol for the document parser worker."""

from __future__ import annotations

import importlib
import json
import traceback
from collections.abc import Callable
from typing import Any, TextIO

PROTOCOL_VERSION = 1


def capability_snapshot() -> dict[str, bool]:
    """Report optional parser backends without making any one of them required."""
    capabilities: dict[str, bool] = {}
    for name, module_name in {
        "docling": "docling.document_converter",
        "rapidocr": "rapid_ocr",
        "api_ocr": "api_ocr",
    }.items():
        try:
            if module_name in {"rapid_ocr", "api_ocr"} and __package__:
                importlib.import_module(f"{__package__}.{module_name}")
            else:
                importlib.import_module(module_name)
        except Exception:  # noqa: BLE001 - optional backends may fail during import
            capabilities[name] = False
        else:
            capabilities[name] = True
    return capabilities


def _write_message(stdout: TextIO, message: dict[str, Any]) -> None:
    stdout.write(json.dumps(message, ensure_ascii=False, separators=(",", ":")) + "\n")
    stdout.flush()


def _error_message(
    request_id: str | None,
    error_code: str,
    error_message: str,
) -> dict[str, Any]:
    message: dict[str, Any] = {
        "type": "result",
        "version": PROTOCOL_VERSION,
        "ok": False,
        "error": {"code": error_code, "message": error_message},
    }
    if request_id:
        message["request_id"] = request_id
    return message


def _request_id(request: Any) -> str | None:
    if isinstance(request, dict) and isinstance(request.get("request_id"), str):
        return request["request_id"]
    return None


def _validate_request(request: Any) -> str | None:
    if not isinstance(request, dict):
        return "request must be a JSON object"
    if request.get("type") != "parse":
        return "request type must be parse"
    if request.get("version") != PROTOCOL_VERSION:
        return "unsupported protocol version"
    for name in ("request_id", "input_path", "filename", "ocr_engine"):
        if not isinstance(request.get(name), str) or not request[name]:
            return f"{name} is required"
    if request["ocr_engine"] not in {"rapidocr", "api"}:
        return "unsupported OCR engine"
    return None


def serve_stdio(
    parse_fn: Callable[[dict[str, Any]], dict[str, Any]],
    stdin: TextIO,
    stdout: TextIO,
    stderr: TextIO,
) -> None:
    """Serve parse requests until EOF, emitting one JSON object per line."""
    _write_message(
        stdout,
        {
            "type": "ready",
            "version": PROTOCOL_VERSION,
            "capabilities": capability_snapshot(),
        },
    )
    for line in stdin:
        if not line.strip():
            continue
        request: Any = None
        try:
            request = json.loads(line)
        except json.JSONDecodeError as exc:
            print(f"invalid protocol JSON: {exc}", file=stderr)
            _write_message(
                stdout,
                _error_message(None, "PARSER_PROTOCOL_ERROR", "invalid protocol request"),
            )
            continue

        request_id = _request_id(request)
        validation_error = _validate_request(request)
        if validation_error:
            print(f"invalid protocol request: {validation_error}", file=stderr)
            _write_message(
                stdout,
                _error_message(request_id, "PARSER_PROTOCOL_ERROR", "invalid protocol request"),
            )
            continue

        try:
            result = parse_fn(request)
            _write_message(
                stdout,
                {
                    "type": "result",
                    "version": PROTOCOL_VERSION,
                    "request_id": request_id,
                    "ok": True,
                    "result": result,
                },
            )
        except Exception as exc:  # noqa: BLE001 - parser failures must not stop the worker
            traceback.print_exc(file=stderr)
            error_code = getattr(exc, "code", "PARSER_FAILED")
            if not isinstance(error_code, str):
                error_code = "PARSER_FAILED"
            _write_message(
                stdout,
                _error_message(request_id, error_code, "document parsing failed"),
            )
