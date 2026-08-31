# Document Ingestion Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a tested document-ingestion pipeline that reuses long-lived Python parser processes, parses digital PDFs with Docling before OCR fallback, conservatively normalizes Markdown, and processes Redis document jobs with bounded concurrency.

**Architecture:** Go owns a fixed-size pool of hidden, long-lived Python subprocesses and communicates with each subprocess through one-request/one-response JSONL messages on stdin/stdout. The existing `ingestion.Parser` continues to handle TXT/Markdown locally, applies a conservative cleaner to every parse result, and delegates binary formats to the pool. DocumentWorker runs a fixed number of Redis consumers, while parse persistence uses a per-job Markdown object key before atomically switching the database record.

**Tech Stack:** Go 1.24+, Redis Streams, Python 3.11, JSONL over OS pipes, Docling, RapidOCR, PyMuPDF, Pillow, MinIO, standard Go testing, Python unittest.

**Spec:** `docs/superpowers/specs/2026-08-31-document-ingestion-pipeline-design.md`

## Global Constraints

- Execute directly on the existing `feature/rag` branch; do not create a new branch or worktree for this implementation.
- Make small, task-scoped commits after each independently passing test cycle; do not squash unrelated tasks into one commit.
- Preserve all unrelated dirty-worktree files; stage only files listed in the current task and never use `git add -A`.
- Keep `DocumentParser.Parse(ctx context.Context, input ParseInput) (ParseResult, error)` as the ingestion boundary.
- Default Python pool size is exactly `2`; default `max_tasks_per_worker` is exactly `100`.
- Do not add a per-document timeout, timeout error code, timeout retry, or timeout process-kill path.
- Keep `--input <path>` CLI compatibility while adding `--serve-stdio`.
- stdout is protocol-only JSONL; all diagnostics go to stderr.
- PDF always tries Docling first and falls back for the whole document only when conversion fails or visible content is empty.
- Cleaning must retain headers, footers, page numbers, `<!-- page:N -->`, code blocks, tables, image URLs, footnotes, versions, confidentiality labels, and disclaimers.
- Do not add LLM cleaning, fuzzy duplicate removal, mixed per-page PDF parsing, a new HTTP service, new parser vendors, auto-indexing, or RAG retrieval changes.
- On Windows, Python workers must start without a visible console window.
- Distinguish unit-tested behavior from real-parser and end-to-end verification in all documentation and handoff notes.

## File Structure

### New Go files

- `internal/ingestion/protocol.go`: JSONL protocol types, validation, and stable protocol errors.
- `internal/ingestion/python_worker.go`: one long-lived Python subprocess and one-at-a-time request execution.
- `internal/ingestion/python_process_windows.go`: Windows no-window process setup and process-tree lifecycle.
- `internal/ingestion/python_process_unix.go`: Linux/macOS process-group lifecycle.
- `internal/ingestion/python_pool.go`: fixed-size pool, worker replacement, one retry for crash/protocol failures, health snapshot.
- `internal/ingestion/cleaner.go`: conservative Markdown normalization.
- `internal/ingestion/testdata/stdio_worker.py`: lightweight deterministic subprocess used by Go pool tests.
- Focused `*_test.go` files beside each new Go component.

### New Python files

- `pkg/documentparser/python/protocol.py`: ready handshake, JSONL request loop, protocol-safe result/error encoding.
- `pkg/documentparser/python/test_protocol.py`: protocol loop tests without real models.
- `pkg/documentparser/python/test_parse_document.py`: mocked PDF routing and fallback tests.
- `pkg/documentparser/python/constraints.txt`: exact direct and critical transitive dependency constraints.
- `pkg/documentparser/python/testdata/`: generated real-format fixtures, expected-content manifest, and generator.

### Modified files

- `pkg/documentparser/python/parse_document.py`: reusable `parse_path`, lazy independent backend imports, PDF Docling-first route, CLI/stdio modes.
- `pkg/documentparser/python/requirements.txt`: use compatible constrained dependencies.
- `internal/ingestion/parser.go`: run the cleaner uniformly after URL replacement.
- `internal/ingestion/types.go`: stable pool/protocol error values and health types where needed.
- `internal/worker/document_worker.go`: bounded consumer concurrency and versioned Markdown persistence.
- `internal/worker/document_worker_test.go`: concurrency and safe object-switch tests.
- `internal/api/v1/ocr/controller.go`: report real pool/backend health.
- `internal/api/v1/ocr/controller_test.go`: health response tests.
- `internal/app/app.go`: construct, inject, and close the pool.
- `pkg/config/config.go`, `pkg/config/config_test.go`, `configs/config.yaml`: pool configuration and defaults.
- `.github/workflows/ci.yml`: lightweight protocol/cleaner tests on every run and a real-parser integration job where dependencies are installed.
- `README.md`, `docs/ARCHITECTURE.md`, `docs/DEVELOPMENT.md`: installation, lifecycle, parser routing, verification boundary.

---

### Task 1: Freeze parser configuration and reproducible dependency inputs

**Files:**
- Modify: `pkg/config/config.go:323-336`
- Modify: `pkg/config/config_test.go`
- Modify: `configs/config.yaml:136-138`
- Modify: `pkg/documentparser/python/requirements.txt`
- Create: `pkg/documentparser/python/constraints.txt`

**Interfaces:**
- Produces: `DocumentParserConfig.PoolSize int`, `DocumentParserConfig.MaxTasksPerWorker int`.
- Produces defaults: `PoolSize=2`, `MaxTasksPerWorker=100`, existing `PythonPath="python"`.
- Consumes: no new runtime components yet.

- [ ] **Step 1: Add failing configuration-default tests**

Append tests that express the exact defaults and explicit-value behavior:

```go
func TestDocumentParserConfigApplyDefaults(t *testing.T) {
    cfg := DocumentParserConfig{}
    cfg.ApplyDefaults()
    if cfg.PythonPath != "python" || cfg.PoolSize != 2 || cfg.MaxTasksPerWorker != 100 {
        t.Fatalf("unexpected defaults: %+v", cfg)
    }
}

func TestDocumentParserConfigPreservesExplicitPoolValues(t *testing.T) {
    cfg := DocumentParserConfig{PythonPath: "py", PoolSize: 4, MaxTasksPerWorker: 20}
    cfg.ApplyDefaults()
    if cfg.PoolSize != 4 || cfg.MaxTasksPerWorker != 20 {
        t.Fatalf("explicit values changed: %+v", cfg)
    }
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run: `go test ./pkg/config -run DocumentParserConfig -count=1 -v`

Expected: FAIL because `PoolSize` and `MaxTasksPerWorker` do not exist.

- [ ] **Step 3: Extend DocumentParserConfig and defaults**

Use these exact fields:

```go
type DocumentParserConfig struct {
    PythonPath        string `mapstructure:"python_path"`
    PoolSize          int    `mapstructure:"pool_size"`
    MaxTasksPerWorker int    `mapstructure:"max_tasks_per_worker"`
}

func (c *DocumentParserConfig) ApplyDefaults() {
    if c.PythonPath == "" {
        c.PythonPath = "python"
    }
    if c.PoolSize <= 0 {
        c.PoolSize = 2
    }
    if c.MaxTasksPerWorker <= 0 {
        c.MaxTasksPerWorker = 100
    }
}
```

Add the two numeric keys with explanatory comments under `document_parser` in `configs/config.yaml`.

- [ ] **Step 4: Constrain the parser dependencies**

Keep `requirements.txt` as the human-readable direct dependency list and add `-c constraints.txt` as its first active line. Create `constraints.txt` with exact direct versions validated in the isolated environment during this task:

```text
docling==2.118.0
rapidocr==3.9.2
onnxruntime==1.28.0
PyMuPDF==1.28.0
Pillow==12.2.0
numpy==1.26.4
opencv-python-headless==4.10.0.84
requests==2.32.5
python-dateutil==2.9.0.post0
packaging==25.0
six==1.17.0
```

Install in a fresh project-local virtual environment and run dependency validation:

```powershell
python -m venv .tmp/document-parser-venv
& .\.tmp\document-parser-venv\Scripts\python.exe -m pip install -r pkg/documentparser/python/requirements.txt
& .\.tmp\document-parser-venv\Scripts\python.exe -m pip check
```

Expected: installation completes and `pip check` reports no broken requirements. If the resolver proves one exact version incompatible, update only the conflicting constraint to the resolver-selected compatible version, repeat from a newly created venv, and record the resolved version in the commit diff; do not validate against the currently broken global Python environment.

- [ ] **Step 5: Run tests and configuration checks**

Run:

```powershell
go test ./pkg/config -count=1
git diff --check -- pkg/config/config.go pkg/config/config_test.go configs/config.yaml pkg/documentparser/python/requirements.txt pkg/documentparser/python/constraints.txt
```

Expected: PASS and no whitespace errors.

- [ ] **Step 6: Commit only Task 1 files**

```powershell
git add -- pkg/config/config.go pkg/config/config_test.go configs/config.yaml pkg/documentparser/python/requirements.txt pkg/documentparser/python/constraints.txt
git commit -m "build(parser): constrain dependencies and pool defaults"
```

---

### Task 2: Add the Python JSONL protocol and reusable parse entry point

**Files:**
- Create: `pkg/documentparser/python/protocol.py`
- Create: `pkg/documentparser/python/test_protocol.py`
- Modify: `pkg/documentparser/python/parse_document.py`

**Interfaces:**
- Produces: `parse_path(path: Path, ocr_engine: str, api_base_url: str, api_key: str, api_model: str) -> ParseResult`.
- Produces: `serve_stdio(parse_fn, stdin, stdout, stderr) -> None`.
- Produces protocol version `1`, message types `ready`, `parse`, and `result`.
- Consumes existing `ParseResult`, OCR functions, image-alt handling.

- [ ] **Step 1: Write failing protocol tests using an injected fake parser**

Cover ready handshake, successful request, safe parser error, malformed JSON, and continued processing after a request-level error. The core success assertion must be:

```python
def test_serve_stdio_emits_ready_and_correlated_result(self) -> None:
    stdin = io.StringIO('{"type":"parse","version":1,"request_id":"r1","input_path":"a.pdf","filename":"a.pdf","ocr_engine":"rapidocr"}\n')
    stdout = io.StringIO()
    stderr = io.StringIO()

    serve_stdio(lambda request: {"markdown": "hello", "metadata": {}}, stdin, stdout, stderr)

    messages = [json.loads(line) for line in stdout.getvalue().splitlines()]
    self.assertEqual(messages[0]["type"], "ready")
    self.assertEqual(messages[1]["request_id"], "r1")
    self.assertTrue(messages[1]["ok"])
```

- [ ] **Step 2: Run protocol tests and verify import/function failures**

Run: `python -m unittest pkg.documentparser.python.test_protocol -v`

Expected: FAIL because `protocol.py` and `serve_stdio` do not exist.

- [ ] **Step 3: Implement the protocol module**

Define request validation and output helpers with these signatures:

```python
PROTOCOL_VERSION = 1

def capability_snapshot() -> dict[str, bool]: ...
def serve_stdio(
    parse_fn: Callable[[dict[str, Any]], dict[str, Any]],
    stdin: TextIO,
    stdout: TextIO,
    stderr: TextIO,
) -> None: ...
```

Write exactly one compact JSON object per stdout line and flush after every message. Validation failures return `PARSER_PROTOCOL_ERROR` correlated to the request ID when one was supplied. Tracebacks and detailed exception text go to stderr only.

- [ ] **Step 4: Extract `parse_path` and preserve CLI behavior**

Refactor format routing out of `main()`:

```python
def parse_path(
    path: Path,
    ocr_engine: str = "rapidocr",
    api_base_url: str = "",
    api_key: str = "",
    api_model: str = "",
) -> ParseResult:
    # validate path/config, route by suffix, return ParseResult
```

Add a mutually exclusive `--serve-stdio` argument. In stdio mode, map each validated request to `parse_path`; in CLI mode, retain the existing `--input` JSON output and exit-code behavior.

- [ ] **Step 5: Make backend imports independent**

Move Docling imports behind `_get_docling_converter()` and OCR imports behind their route/factory. `protocol.capability_snapshot()` catches imports separately so a Docling import failure yields `docling:false` without preventing a worker ready message or RapidOCR parsing.

- [ ] **Step 6: Run Python tests and syntax checks**

Run:

```powershell
python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
python -m py_compile pkg/documentparser/python/parse_document.py pkg/documentparser/python/protocol.py
```

Expected: all lightweight tests PASS without loading an OCR model.

- [ ] **Step 7: Commit only Task 2 files**

```powershell
git add -- pkg/documentparser/python/protocol.py pkg/documentparser/python/test_protocol.py pkg/documentparser/python/parse_document.py
git commit -m "feat(parser): add persistent JSONL runtime"
```

---

### Task 3: Route digital PDFs through Docling with whole-document OCR fallback

**Files:**
- Modify: `pkg/documentparser/python/parse_document.py`
- Create: `pkg/documentparser/python/test_parse_document.py`

**Interfaces:**
- Consumes: `parse_path(...) -> ParseResult` from Task 2.
- Produces: `_parse_pdf(path, result, recognizer settings) -> str` with metadata `parser`, `fallback`, and optional `fallback_reason`.
- Preserves existing `pages` output for OCR fallback.

- [ ] **Step 1: Write mocked failing tests for the three PDF outcomes**

Add tests for Docling success, Docling empty fallback, and Docling exception fallback. The success and empty assertions must include:

```python
def test_pdf_prefers_docling_when_visible_content_exists(self) -> None:
    with patch("parse_document._convert_with_docling", return_value="# 数字版标题\n正文 2026") as docling, \
         patch("parse_document.ocr_pdf") as ocr:
        result = parse_path(Path("digital.pdf"))
    self.assertEqual(result.metadata, {"file_type": ".pdf", "parser": "docling", "fallback": False})
    ocr.assert_not_called()

def test_pdf_falls_back_when_docling_is_empty(self) -> None:
    with patch("parse_document._convert_with_docling", return_value="  #  \n"), \
         patch("parse_document.ocr_pdf", return_value=("扫描正文", [{"number": 1, "text": "扫描正文"}])):
        result = parse_path(Path("scan.pdf"))
    self.assertTrue(result.metadata["fallback"])
    self.assertEqual(result.metadata["fallback_reason"], "docling_empty")
```

Patch `Path.is_file` or create a temporary empty path so tests exercise routing without requiring real files.

- [ ] **Step 2: Run the focused tests and verify current OCR-first behavior fails**

Run: `python -m unittest pkg.documentparser.python.test_parse_document -v`

Expected: FAIL because PDF currently calls OCR directly.

- [ ] **Step 3: Implement conservative visible-content detection**

```python
VISIBLE_TEXT_PATTERN = re.compile(r"[A-Za-z0-9\u3400-\u9fff]")

def has_visible_content(markdown: str) -> bool:
    without_markup = re.sub(r"[`#>*_\[\]()!|\-]", "", markdown or "")
    return VISIBLE_TEXT_PATTERN.search(without_markup) is not None
```

This check only detects a completely empty/non-content result; do not introduce quality scoring.

- [ ] **Step 4: Implement Docling-first routing and safe fallback metadata**

Use the existing Docling converter for PDF as well as Office. On Docling exception, write the detailed exception to stderr and set `fallback_reason="docling_failed"`; on empty content use `docling_empty`. Call exactly one OCR backend for the entire PDF.

- [ ] **Step 5: Run focused and complete lightweight Python tests**

Run:

```powershell
python -m unittest pkg.documentparser.python.test_parse_document -v
python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
```

Expected: PASS.

- [ ] **Step 6: Commit only Task 3 files**

```powershell
git add -- pkg/documentparser/python/parse_document.py pkg/documentparser/python/test_parse_document.py
git commit -m "feat(parser): prefer Docling for digital PDFs"
```

---

### Task 4: Implement one long-lived Go PythonWorker and platform process control

**Files:**
- Create: `internal/ingestion/protocol.go`
- Create: `internal/ingestion/python_worker.go`
- Create: `internal/ingestion/python_process_windows.go`
- Create: `internal/ingestion/python_process_unix.go`
- Create: `internal/ingestion/python_worker_test.go`
- Create: `internal/ingestion/testdata/stdio_worker.py`
- Modify: `internal/ingestion/types.go`

**Interfaces:**
- Produces: `type PythonWorkerOptions struct { PythonPath, ScriptPath string; MaxTasks int; OCR OCRConfig; ExtraEnv []string }` (`ExtraEnv` is used by deterministic subprocess tests and remains empty in production wiring).
- Produces: `startPythonWorker(ctx context.Context, opts PythonWorkerOptions) (*PythonWorker, error)`.
- Produces: `func (w *PythonWorker) Parse(ctx context.Context, input ParseInput) (ParseResult, error)` and `func (w *PythonWorker) Close() error`.
- Produces sentinel errors `ErrPythonWorkerCrashed` and `ErrPythonProtocol`.

- [ ] **Step 1: Create a deterministic stdio test worker**

The helper supports request operations selected by filename:

```python
if filename == "crash.pdf":
    os._exit(17)
if filename == "bad-json.pdf":
    print("not-json", flush=True)
else:
    print(json.dumps({"type":"result","version":1,"request_id":request["request_id"],"ok":True,"result":{"markdown":f"pid={os.getpid()} name={filename}"}}), flush=True)
```

It must emit a valid ready message before reading requests and write a diagnostic line to stderr for the normal path.

- [ ] **Step 2: Write failing Go worker tests**

Cover ready handshake, two requests returning the same PID, stderr draining, request-ID mismatch, invalid JSON, process exit, and Close. Use `exec.LookPath("python")`; skip only when Python truly is unavailable.

```go
func TestPythonWorkerReusesProcess(t *testing.T) {
    worker := startTestWorker(t)
    defer worker.Close()
    first, err := worker.Parse(context.Background(), ParseInput{Filename: "one.pdf"})
    if err != nil { t.Fatal(err) }
    second, err := worker.Parse(context.Background(), ParseInput{Filename: "two.pdf"})
    if err != nil { t.Fatal(err) }
    if pidFrom(first.Markdown) != pidFrom(second.Markdown) {
        t.Fatalf("worker process was not reused: %q / %q", first.Markdown, second.Markdown)
    }
}
```

- [ ] **Step 3: Run the focused test and verify missing types fail**

Run: `go test ./internal/ingestion -run PythonWorker -count=1 -v`

Expected: FAIL because PythonWorker is undefined.

- [ ] **Step 4: Define and validate protocol messages**

Use typed envelopes rather than `map[string]any`:

```go
const parserProtocolVersion = 1

type parserRequest struct {
    Type        string `json:"type"`
    Version     int    `json:"version"`
    RequestID   string `json:"request_id"`
    InputPath   string `json:"input_path"`
    Filename    string `json:"filename"`
    OCREngine   string `json:"ocr_engine"`
    OCRAPIURL   string `json:"ocr_api_url,omitempty"`
    OCRAPIKey   string `json:"ocr_api_key,omitempty"`
    OCRAPIModel string `json:"ocr_api_model,omitempty"`
}

type parserResponse struct {
    Type      string      `json:"type"`
    Version   int         `json:"version"`
    RequestID string      `json:"request_id"`
    OK        bool        `json:"ok"`
    Result    ParseResult `json:"result"`
    Error     *struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error,omitempty"`
}
```

Add validation helpers that reject non-result messages, wrong versions, mismatched IDs, missing results, and unsafe empty error fields with `ErrPythonProtocol`.

- [ ] **Step 5: Implement PythonWorker lifecycle**

Start `python <script> --serve-stdio`, capture all three pipes, drain stderr in a goroutine, decode one ready message, and guard `Parse` so one request is in flight. For each Parse, create a local temporary directory, write `input.Content` to `filepath.Base(input.Filename)`, send the local path, decode one response, upload/replace picture paths before removing the directory, and increment the task count only after a response.

Context cancellation is used for Qavor shutdown, not as a newly added document timeout. On cancellation, close/terminate the worker so a blocked pipe read can exit.

- [ ] **Step 6: Implement platform process setup**

Windows file:

```go
//go:build windows

func configureParserProcess(cmd *exec.Cmd) error {
    cmd.SysProcAttr = &syscall.SysProcAttr{
        HideWindow: true,
        CreationFlags: 0x08000000, // CREATE_NO_WINDOW
    }
    return nil
}

func attachParserProcess(cmd *exec.Cmd) (parserProcessController, error) {
    return attachKillOnCloseJob(cmd.Process.Pid)
}
```

Call `configureParserProcess` before `cmd.Start`, then call `attachParserProcess` only after Start succeeds. Use `golang.org/x/sys/windows` Job Object APIs with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, assign the started PID to the job, and close the job handle during forced shutdown. Linux/macOS use a process group (`Setpgid: true`) and return a controller that signals the group during forced shutdown. Normal Close first closes stdin and waits for the process to exit before forcing termination.

- [ ] **Step 7: Run tests on the current platform and cross-compile process files**

Run:

```powershell
go test ./internal/ingestion -run PythonWorker -count=1 -v
$env:GOOS='windows'; go test ./internal/ingestion -run '^$'; Remove-Item Env:GOOS
$env:GOOS='linux'; go test ./internal/ingestion -run '^$'; Remove-Item Env:GOOS
```

Expected: worker tests PASS; both target packages compile. Cross-compiled test binaries need not execute.

- [ ] **Step 8: Commit only Task 4 files and required module changes**

If `golang.org/x/sys` changes `go.mod/go.sum`, include only those module files with the Task 4 files.

```powershell
git add -- internal/ingestion/protocol.go internal/ingestion/python_worker.go internal/ingestion/python_process_windows.go internal/ingestion/python_process_unix.go internal/ingestion/python_worker_test.go internal/ingestion/testdata/stdio_worker.py internal/ingestion/types.go go.mod go.sum
git commit -m "feat(ingestion): add persistent Python worker"
```

---

### Task 5: Implement PythonWorkerPool, replacement, retry, and health

**Files:**
- Create: `internal/ingestion/python_pool.go`
- Create: `internal/ingestion/python_pool_test.go`
- Modify: `internal/ingestion/python_worker.go`

**Interfaces:**
- Consumes: PythonWorker and errors from Task 4.
- Produces: `NewPythonWorkerPool(ctx context.Context, opts PythonPoolOptions) (*PythonWorkerPool, error)`.
- Produces: `func (p *PythonWorkerPool) Parse(ctx context.Context, input ParseInput) (ParseResult, error)`.
- Produces: `func (p *PythonWorkerPool) Health() ParserHealth` and `func (p *PythonWorkerPool) Close() error`.

- [ ] **Step 1: Write failing fixed-capacity and reuse tests**

Define exact options and health shapes in the tests:

```go
type PythonPoolOptions struct {
    Worker PythonWorkerOptions
    Size int
}

type ParserHealth struct {
    ConfiguredWorkers int
    AvailableWorkers int
    Capabilities map[string]bool
}
```

Test size 2, two simultaneous requests, a third request waiting until a worker is returned, stable PIDs across sequential requests, and Close rejection.

- [ ] **Step 2: Write failing crash/protocol retry tests**

Extend the test helper so `crash-once.pdf` and `bad-json-once.pdf` store their one-shot marker under the directory supplied by `QAVOR_TEST_STATE_DIR`. Create that directory with `t.TempDir()` and pass it through `PythonWorkerOptions.ExtraEnv`; it remains shared when the pool creates a replacement process. Assert the pool replaces the bad worker and succeeds after exactly one retry. Assert `always-crash.pdf` returns after two attempts and does not loop.

- [ ] **Step 3: Run focused tests and verify missing pool failures**

Run: `go test ./internal/ingestion -run PythonWorkerPool -count=1 -v`

Expected: FAIL because PythonWorkerPool does not exist.

- [ ] **Step 4: Implement fixed-size pool and centralized replacement**

Use a buffered channel of idle workers plus a registry protected by a mutex. A replacement path removes and closes the bad worker, starts one replacement, registers it, and returns it to the idle channel unless the pool is closing.

Reject `Size<=0` and `MaxTasks<=0` during construction. If only some configured workers start, return a live degraded pool when at least one worker is available; fail construction when zero start.

- [ ] **Step 5: Implement exactly-one infrastructure retry**

Retry only when `errors.Is(err, ErrPythonWorkerCrashed)` or `errors.Is(err, ErrPythonProtocol)`. ParserError and unsupported/corrupt document errors return immediately. Retire a worker after `MaxTasks` completed requests and replace it only after it becomes idle.

- [ ] **Step 6: Implement race-safe health and Close**

Health snapshots copy the capabilities map and never expose mutable pool state. Close marks the pool closed, prevents replacement, closes idle and leased workers as they return, and is idempotent.

- [ ] **Step 7: Run focused tests under the race detector**

Run:

```powershell
go test -race ./internal/ingestion -run 'PythonWorker(Pool)?' -count=1 -v
go test ./internal/ingestion -count=1
```

Expected: PASS with no race report and no residual helper Python process owned by the tests.

- [ ] **Step 8: Commit only Task 5 files**

```powershell
git add -- internal/ingestion/python_pool.go internal/ingestion/python_pool_test.go internal/ingestion/python_worker.go
git commit -m "feat(ingestion): add managed Python worker pool"
```

---

### Task 6: Wire pool lifecycle, real health, and bounded Redis consumers

**Files:**
- Modify: `internal/app/app.go:160-180,360-465,768-838`
- Modify: `internal/api/v1/ocr/controller.go`
- Create: `internal/api/v1/ocr/controller_test.go`
- Modify: `internal/worker/document_worker.go:22-29,319-397`
- Modify: `internal/worker/document_worker_test.go`

**Interfaces:**
- Consumes: `PythonWorkerPool`, `ParserHealth`, and config fields from Tasks 1 and 5.
- Produces: `DocumentWorkerOptions.ConsumerCount int`.
- Produces: optional `ParserHealthProvider` accepted by the OCR controller.

- [ ] **Step 1: Write failing worker-concurrency tests**

Use a channel-backed fake queue that records consumer IDs. Configure `ConsumerCount: 2`, publish two blocking fake parse jobs, and assert both enter processing before either is released. Assert consumer IDs end in distinct `-0` and `-1` suffixes. Add a test that the pending recovery loop is started once, not once per consumer.

- [ ] **Step 2: Write failing OCR health tests**

Define:

```go
type ParserHealthProvider interface {
    Health() ingestion.ParserHealth
}
```

Test healthy (2/2), degraded (1/2 or missing Docling capability), and unavailable (0/2). Keep existing API OCR configuration checks, but make `rapid_ocr` availability depend on the real capability snapshot instead of a static string.

- [ ] **Step 3: Run focused tests and verify failures**

Run:

```powershell
go test ./internal/worker -run 'Concurrent|Consumer' -count=1 -v
go test ./internal/api/v1/ocr -count=1 -v
```

Expected: FAIL because the consumer count and health provider are not wired.

- [ ] **Step 4: Implement bounded consumer loops**

Add `ConsumerCount int` and default it to 1 inside `applyDefaults` for compatibility. `Run` calls `EnsureGroup` once, starts exactly `ConsumerCount` loops with IDs `workerID-0` through `workerID-(n-1)`, starts one recovery coordinator, and waits for all loops on shutdown. Do not spawn one recovery loop per consumer.

- [ ] **Step 5: Wire pool construction and application shutdown**

Add a pool field to `App` so graceful shutdown can close it. Build the pool only when the document queue is available, pass the existing OCR settings and image uploader through `PythonPoolOptions`, inject the pool into `ingestion.NewParser`, and set `ConsumerCount` from `DocumentParser.PoolSize`. Move OCR-controller construction to after this pool initialization so `ocrctrl.NewController(systemConfigSvc, parserPool)` receives the live provider; when the queue or pool is unavailable, construct it without a provider and report the parser pool unavailable.

Shutdown order:

```text
cancel DocumentWorker consumers
→ wait for worker loops to exit
→ close PythonWorkerPool
→ continue existing service shutdown
```

This cancellation is lifecycle control, not a document timeout.

- [ ] **Step 6: Wire real health into the controller**

Change the constructor to accept the optional parser-health provider without breaking tests that omit it:

```go
func NewController(ocrCfg OCRConfigProvider, parserHealth ...ParserHealthProvider) *Controller
```

Return existing per-engine health plus a `parser_pool` object containing configured/available workers and capability booleans.

- [ ] **Step 7: Run worker, API, app, and race tests**

Run:

```powershell
go test -race ./internal/worker ./internal/ingestion -count=1
go test ./internal/api/v1/ocr ./internal/app -count=1
go build ./cmd/server
```

Expected: PASS.

- [ ] **Step 8: Commit only Task 6 files**

```powershell
git add -- internal/app/app.go internal/api/v1/ocr/controller.go internal/api/v1/ocr/controller_test.go internal/worker/document_worker.go internal/worker/document_worker_test.go
git commit -m "feat(worker): run bounded parser consumers"
```

---

### Task 7: Add conservative cleaning and safe Markdown object switching

**Files:**
- Create: `internal/ingestion/cleaner.go`
- Create: `internal/ingestion/cleaner_test.go`
- Modify: `internal/ingestion/parser.go`
- Modify: `internal/ingestion/parser_test.go`
- Modify: `internal/worker/document_worker.go:103-157`
- Modify: `internal/worker/document_worker_test.go`

**Interfaces:**
- Produces: `type CleanResult struct { Markdown string; Degraded bool }`.
- Produces: `func (DocumentCleaner) Clean(markdown string) CleanResult`.
- Consumes: ParseResult from text parsing and PythonWorkerPool.

- [ ] **Step 1: Write failing cleaner golden tests**

Create table-driven tests that prove exact normalization and preservation. Include this preservation case verbatim:

```go
input := "公司内部资料 版本 2\r\n<!-- page:1 -->\r\n# 标题\r\n\r\n|列A|列B|\r\n|-|-|\r\n|1|2|\r\n\r\n![图表文字](https://objects.test/a.png)\r\n第 1 页 / 共 2 页"
got := (DocumentCleaner{}).Clean(input)
for _, want := range []string{"公司内部资料 版本 2", "<!-- page:1 -->", "|1|2|", "https://objects.test/a.png", "第 1 页 / 共 2 页"} {
    if !strings.Contains(got.Markdown, want) { t.Fatalf("missing preserved content %q", want) }
}
```

Add exact cases for control-character removal, trailing spaces, at most one blank line, empty headings, empty parser image placeholders, code fences, footnotes, and disclaimers.

- [ ] **Step 2: Run cleaner tests and verify missing type failure**

Run: `go test ./internal/ingestion -run DocumentCleaner -count=1 -v`

Expected: FAIL because DocumentCleaner does not exist.

- [ ] **Step 3: Implement rule-isolated cleaning**

Use small deterministic rules:

```go
type cleanRule func(string) (string, error)

type CleanResult struct {
    Markdown string
    Degraded bool
}

func (DocumentCleaner) Clean(markdown string) CleanResult {
    result := CleanResult{Markdown: markdown}
    for _, rule := range []cleanRule{normalizeNewlines, removeUnsafeControls, trimLineEnds, removeEmptyArtifacts, collapseBlankLines} {
        next, err := rule(result.Markdown)
        if err != nil { result.Degraded = true; continue }
        result.Markdown = next
    }
    return result
}
```

Rules must not recognize or remove headers, footers, page numbers, or fuzzy duplicates.

- [ ] **Step 4: Apply the cleaner uniformly in Parser**

Refactor `Parser.Parse` so every successful branch passes through one finalizer. Initialize `Metadata` if nil and set `cleaner_degraded=true` only when degraded. If the pre-clean result has no visible content, return `ParserError{Code:"PARSER_EMPTY_CONTENT", Message:"文档解析结果为空"}`; never turn non-empty input into a failure solely because a cleaner rule failed.

- [ ] **Step 5: Write failing versioned-object persistence tests**

Extend fake storage to record upload filenames and deletes. Assert parse job `job-123` uploads `normalized-job-123.md`, transitions the database record to the new object path, then best-effort deletes the prior Markdown path only after the transition succeeds. Assert transition failure deletes the newly uploaded object and never deletes the old one.

- [ ] **Step 6: Implement safe object switching**

Capture `oldMarkdown := file.MarkdownFile`; upload to:

```go
filename := fmt.Sprintf("normalized-%s.md", job.JobID)
```

If status transition fails, delete the new object best-effort before returning. After transition and job success, delete the old object best-effort when it is non-empty and differs from the new path. Log cleanup failure without reverting the successful switch.

- [ ] **Step 7: Run focused and package tests**

Run:

```powershell
go test ./internal/ingestion -run 'DocumentCleaner|Parse' -count=1 -v
go test ./internal/worker -run 'ParseJob|Markdown' -count=1 -v
go test ./internal/ingestion ./internal/worker -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit only Task 7 files**

```powershell
git add -- internal/ingestion/cleaner.go internal/ingestion/cleaner_test.go internal/ingestion/parser.go internal/ingestion/parser_test.go internal/worker/document_worker.go internal/worker/document_worker_test.go
git commit -m "feat(ingestion): normalize parsed Markdown safely"
```

---

### Task 8: Add generated real-format fixtures and real-parser integration tests

**Files:**
- Create: `pkg/documentparser/python/testdata/generate_fixtures.py`
- Create: `pkg/documentparser/python/testdata/README.md`
- Create: `pkg/documentparser/python/testdata/expected.json`
- Create generated fixtures: `plain.txt`, `structured.md`, `digital.pdf`, `scanned.pdf`, `with-image.docx`, `slides.pptx`, `table.xlsx`, `text-image.png`, `corrupted.pdf`
- Create: `pkg/documentparser/python/test_integration.py`
- Create: `pkg/documentparser/python/requirements-test.txt`

**Interfaces:**
- Consumes: Python `parse_path`, stdio protocol, and constrained environment from Tasks 1-3.
- Produces: reproducible committed fixtures and assertions used in final acceptance.

- [ ] **Step 1: Define the expected-content manifest**

Create exact expected markers:

```json
{
  "digital.pdf": ["Qavor 数字 PDF", "结构化表格", "2026"],
  "scanned.pdf": ["Qavor 扫描 PDF", "OCR 314159"],
  "with-image.docx": ["DOCX 正文", "图片文字 271828"],
  "slides.pptx": ["PPTX 标题", "演示正文"],
  "table.xlsx": ["产品", "数量", "42"],
  "text-image.png": ["图片 OCR", "161803"]
}
```

- [ ] **Step 2: Create a deterministic fixture generator**

Use PyMuPDF and Pillow for both PDFs and the PNG; use `python-docx`, `python-pptx`, and `openpyxl` for Office fixtures. Set fixed document metadata and timestamps where the libraries allow it. Generate all files under the script directory and document that the content is synthetic and CC0/project-owned.

`requirements-test.txt` contains:

```text
-c constraints.txt
python-docx==1.2.0
python-pptx==1.0.2
openpyxl==3.1.5
```

- [ ] **Step 3: Generate fixtures and verify deterministic expected content**

Run in the isolated parser venv:

```powershell
& .\.tmp\document-parser-venv\Scripts\python.exe -m pip install -r pkg/documentparser/python/requirements-test.txt
& .\.tmp\document-parser-venv\Scripts\python.exe pkg/documentparser/python/testdata/generate_fixtures.py
```

Run the generator twice and compare SHA-256 hashes. If an Office library writes nondeterministic ZIP timestamps, have the generator normalize ZIP entry timestamps before the second comparison. Expected: hashes are identical.

- [ ] **Step 4: Write real-parser integration tests**

Mark the suite with `@unittest.skipUnless(os.getenv("QAVOR_REAL_PARSER_TESTS") == "1", ...)`. Test:

- digital PDF uses `parser=docling`, `fallback=false`, and expected markers;
- scanned PDF has `fallback=true` and OCR parser metadata;
- Office and PNG contain expected markers;
- DOCX image OCR text appears in Markdown alt text;
- corrupted PDF returns the stable safe parser error;
- two stdio requests are answered by the same Python PID (add `worker_pid` to result metadata in serve mode only).

- [ ] **Step 5: Run lightweight and real suites separately**

Run:

```powershell
python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
$env:QAVOR_REAL_PARSER_TESTS='1'
& .\.tmp\document-parser-venv\Scripts\python.exe -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
Remove-Item Env:QAVOR_REAL_PARSER_TESTS
```

Expected: lightweight suite passes in the default environment; real suite passes in the isolated environment. If model download is required, record that as real-runtime evidence rather than weakening assertions.

- [ ] **Step 6: Commit only Task 8 files**

```powershell
git add -- pkg/documentparser/python/testdata pkg/documentparser/python/test_integration.py pkg/documentparser/python/requirements-test.txt
git commit -m "test(parser): add real document fixtures"
```

---

### Task 9: Add CI lanes, documentation, and final end-to-end acceptance

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/DEVELOPMENT.md`
- Modify: `docs/API.md`
- Test/verify: all files from Tasks 1-8

**Interfaces:**
- Consumes: completed implementation and fixtures.
- Produces: repeatable CI verification and accurate operator/developer documentation.

- [ ] **Step 1: Add lightweight Python CI after backend tests**

Add a job or backend steps that install only what the lightweight protocol tests need and run:

```yaml
- name: Run lightweight document parser tests
  run: python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
```

Keep real model integration separate so ordinary Go/unit CI does not silently download large models.

- [ ] **Step 2: Add an explicit real-parser integration job**

Use Python 3.11, install `requirements.txt` plus `requirements-test.txt`, set `QAVOR_REAL_PARSER_TESTS=1`, and run the Python suite. Give this job a clear name and a CI timeout; the application itself still receives no document task timeout.

- [ ] **Step 3: Update documentation with exact boundaries**

Document:

- the pool defaults and configuration keys;
- hidden local subprocess communication and no network port;
- digital PDF Docling-first behavior and whole-document OCR fallback;
- conservative cleaning that retains headers/footers/page numbers;
- no per-document timeout and the known stuck-worker limitation;
- isolated Python installation commands using constraints;
- distinction between lightweight tests, real-parser tests, and full service acceptance;
- health response meaning and application shutdown behavior.

- [ ] **Step 4: Run all static and package-level verification**

Run:

```powershell
gofmt -w internal/ingestion internal/worker internal/api/v1/ocr internal/app pkg/config
go test -race ./internal/ingestion ./internal/worker -count=1
go test ./... -count=1
go vet ./...
go build ./cmd/server
python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
git diff --check
```

Expected: every command passes. Preserve unrelated dirty files during formatting by formatting only the Go files changed by this implementation if unrelated files exist in any listed directory.

- [ ] **Step 5: Run real-parser verification in the isolated environment**

Run:

```powershell
$env:QAVOR_REAL_PARSER_TESTS='1'
& .\.tmp\document-parser-venv\Scripts\python.exe -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
Remove-Item Env:QAVOR_REAL_PARSER_TESTS
```

Expected: digital PDF, scanned PDF, Office, image, protocol reuse, and corrupted-file assertions all pass.

- [ ] **Step 6: Run full service acceptance when PostgreSQL, Redis, and MinIO are available**

Upload the committed digital PDF, scanned PDF, DOCX-with-image, and PNG fixtures through the real API. For each file verify:

```text
parse_queued → parsing → parsed
manual index → index_queued → indexing → indexed
query retrieves the unique expected marker
```

Record actual file IDs, parser metadata, Markdown object paths, chunk counts, and query hits in the implementation handoff. If external services are unavailable, report this step as pending rather than claiming runtime verification.

- [ ] **Step 7: Verify process reuse and shutdown on Windows**

With Qavor running, submit at least four binary fixtures and record Python worker PIDs before and after. Expected: at most `pool_size` long-lived workers, reused across requests; no visible console window. Stop Qavor and verify only the exact PIDs started by this run have exited—do not terminate unrelated Python processes.

- [ ] **Step 8: Commit documentation and CI only**

```powershell
git add -- .github/workflows/ci.yml README.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/API.md
git commit -m "docs(parser): document pooled ingestion verification"
```

---

## Final Review Checklist

- [ ] Every spec acceptance criterion maps to a task and a test or explicit runtime check.
- [ ] No per-document timeout behavior was introduced accidentally.
- [ ] Parser stdout contains JSONL only; logs and tracebacks use stderr.
- [ ] Infrastructure failures retry at most once; ordinary parse failures do not retry.
- [ ] Headers, footers, page numbers, page markers, tables, code blocks, image URLs, and disclaimers remain in cleaned Markdown.
- [ ] Digital PDF uses Docling and scanned PDF uses whole-document OCR fallback.
- [ ] Pool and Redis consumer concurrency are both bounded.
- [ ] Windows subprocesses are hidden and application-owned process trees close on shutdown.
- [ ] Versioned Markdown upload does not overwrite the active object before the database switch.
- [ ] Real-parser claims are backed by the isolated integration suite; service claims are backed by PostgreSQL/Redis/MinIO acceptance evidence.
- [ ] Each commit stages only task-owned files and excludes all unrelated worktree changes.
