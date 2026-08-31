#!/usr/bin/env python3
"""Small deterministic JSONL worker used only by Go subprocess tests."""

import json
import os
import sys
import time


def response(request):
    filename = request["filename"]
    if filename == "crash.pdf":
        os._exit(17)
    if filename == "block.pdf":
        marker = os.getenv("QAVOR_TEST_BLOCK_MARKER")
        if marker:
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write("blocked")
        while True:
            time.sleep(1)
    if filename == "bad-json.pdf":
        print("not-json", flush=True)
        return
    request_id = request["request_id"]
    if filename == "wrong-request-id.pdf":
        request_id = "another-request"
    if filename == "long-stderr.pdf":
        sys.stderr.write("x" * (2 * 1024 * 1024))
        sys.stderr.flush()
    print(
        json.dumps(
            {
                "type": "result",
                "version": 1,
                "request_id": request_id,
                "ok": True,
                "result": {"markdown": f"pid={os.getpid()} name={filename}"},
            }
        ),
        flush=True,
    )
    if filename == "stderr.pdf":
        print("diagnostic: " + ("x" * 200_000), file=sys.stderr, flush=True)
    else:
        print(f"diagnostic: parsed {filename}", file=sys.stderr, flush=True)


def main():
    if os.getenv("QAVOR_STDIO_MODE") == "no-ready":
        marker = os.getenv("QAVOR_TEST_READY_MARKER")
        if marker:
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write("waiting")
        exit_after = float(os.getenv("QAVOR_TEST_EXIT_AFTER", "0"))
        if exit_after:
            time.sleep(exit_after)
            return
        while True:
            time.sleep(1)
    print(
        json.dumps(
            {
                "type": "ready",
                "version": 1,
                "capabilities": {"docling": False, "rapidocr": False, "api_ocr": False},
            }
        ),
        flush=True,
    )
    for line in sys.stdin:
        response(json.loads(line))


if __name__ == "__main__":
    main()
