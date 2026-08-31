#!/usr/bin/env python3
"""Small deterministic JSONL worker used only by Go subprocess tests."""

import json
import os
import sys


def response(request):
    filename = request["filename"]
    if filename == "crash.pdf":
        os._exit(17)
    if filename == "bad-json.pdf":
        print("not-json", flush=True)
        return
    request_id = request["request_id"]
    if filename == "wrong-request-id.pdf":
        request_id = "another-request"
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
