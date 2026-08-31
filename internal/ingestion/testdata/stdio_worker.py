#!/usr/bin/env python3
"""Small deterministic JSONL worker used only by Go subprocess tests."""

import json
import os
import sys
import time


def response(request):
    filename = request["filename"]
    state_dir = os.getenv("QAVOR_TEST_STATE_DIR", "")

    def state_path():
        if not state_dir:
            return ""
        os.makedirs(state_dir, exist_ok=True)
        return os.path.join(state_dir, filename)

    if filename == "wait.pdf":
        marker = state_path()
        if marker:
            with open(os.path.join(state_dir, f"wait-{os.getpid()}"), "w", encoding="utf-8") as marker_file:
                marker_file.write("started")
        release = os.getenv("QAVOR_TEST_RELEASE_FILE", "")
        while release and not os.path.exists(release):
            time.sleep(0.01)
    if filename in {"crash-once.pdf", "bad-json-once.pdf"}:
        marker = state_path()
        if marker and not os.path.exists(marker):
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write("1")
            if filename == "crash-once.pdf":
                os._exit(17)
            print("not-json", flush=True)
            return
    if filename == "always-crash.pdf":
        marker = state_path()
        attempts = 0
        if marker and os.path.exists(marker):
            with open(marker, encoding="utf-8") as marker_file:
                attempts = int(marker_file.read())
        if marker:
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write(str(attempts + 1))
        os._exit(17)
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
    if filename == "parser-error.pdf":
        print(
            json.dumps(
                {
                    "type": "result",
                    "version": 1,
                    "request_id": request_id,
                    "ok": False,
                    "error": {"code": "PARSER_FAILED", "message": "bad input"},
                }
            ),
            flush=True,
        )
        return
    # The Go race test closes stdin after this response has started but before it finishes.
    if filename == "close-race.pdf":
        payload = json.dumps(
            {
                "type": "result",
                "version": 1,
                "request_id": request_id,
                "ok": True,
                "result": {"markdown": "x" * (4 * 1024 * 1024)},
            }
        )
        midpoint = len(payload) // 2
        sys.stdout.write(payload[:midpoint])
        sys.stdout.flush()
        marker = os.getenv("QAVOR_TEST_CLOSE_RACE_MARKER")
        if marker:
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write("response-started")
        sys.stdout.write(payload[midpoint:] + "\n")
        sys.stdout.flush()
        return
    if filename == "normal-close-race.pdf":
        marker = os.getenv("QAVOR_TEST_NORMAL_CLOSE_MARKER")
        if marker:
            with open(marker, "w", encoding="utf-8") as marker_file:
                marker_file.write("request-received")
        print(
            json.dumps(
                {
                    "type": "result",
                    "version": 1,
                    "request_id": request_id,
                    "ok": True,
                    "result": {"markdown": "x" * (2 * 1024 * 1024)},
                }
            ),
            flush=True,
        )
        return
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
