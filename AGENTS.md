# Qavor Agent Instructions

## Project map

Qavor is a full-stack platform for building, running, and observing AI Agents.

- `cmd/server/main.go` starts the Go/Gin API server.
- `internal/app/app.go` is the composition root. Follow wiring from there before
  changing Agent, RAG, queue, parser, MCP, Skill, or Trace behavior.
- `internal/api/v1/` contains HTTP handlers; business logic belongs in
  `internal/service/` and persistence in `internal/repository/`.
- `internal/agent/` contains Agent construction, prompts, tools, middleware,
  subagents, and streaming behavior. `internal/run/executor.go` executes an
  Agent run.
- `internal/rag/` contains retrieval and answer components. The direct
  `POST /api/v1/rag/answer` endpoint is a RAG API, not an Agent-tool decision
  path.
- `internal/worker/` runs asynchronous work. Document parsing enters through
  `pkg/documentparser/python/parse_document.py`; its Go wiring is in
  `internal/app/app.go` and `internal/worker/document_worker.go`.
- `frontend/` is the Vue 3/Vite application. Its scripts are defined in
  `frontend/package.json`.

## Working rules

- Inspect the actual caller, API route, service, repository, and worker/runtime
  wiring before changing behavior. Do not report static inspection as a live
  runtime result.
- Keep changes scoped. Reuse existing patterns and dependencies; do not add a
  package or an abstraction for one use.
- Preserve unrelated working-tree changes. Do not add, delete, or rewrite
  unrequested files.
- Treat `.env`, `configs/config.yaml`, API keys, passwords, JWT secrets, and
  object-storage credentials as secrets. Never commit, print, or replace them.
- Do not run migrations, destructive cleanup, Docker changes, or external API
  calls unless the task explicitly requires them.

## Agent and RAG behavior

- `query_kb` is available only when an Agent has knowledge-base bindings.
  Use it for uploaded, internal, project, or organization-specific facts and
  requests for source citations.
- Do not force `query_kb` for greetings, rewriting, translation, supplied-text
  summaries, calculations, or general knowledge. Do not repeat it unless the
  query is materially clarified, split, or constrained.
- A tool registry or prompt rule is not proof that a tool ran. Trace the
  executor and runtime evidence before making that claim.
- Keep Agent tool routing separate from direct RAG endpoints and from MCP tool
  filtering.

## Commands

Run the narrowest relevant check first, then expand only when the change spans
multiple layers.

```powershell
# Go backend
go test ./internal/agent ./internal/run ./internal/service ./internal/rag
go test ./...
go build ./cmd/server

# Vue frontend (run from frontend/)
pnpm test:unit
pnpm build

# Python document parser (after installing its test requirements)
python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
```

On restricted Windows environments, use repository-local `GOPATH`, `GOCACHE`,
and `GOMODCACHE` if the default Go cache is not writable. Report environment
blocks separately from code failures.

## Change-specific checks

- Agent prompt/tool changes: run relevant tests in `internal/agent/` and
  `internal/run/`.
- Retrieval or answer-chain changes: run relevant `internal/rag/` and
  `internal/service/` tests. Live embeddings, rerankers, PostgreSQL extensions,
  and model credentials require separate runtime verification.
- Parser/OCR changes: run the Python suite. Real-parser integration tests are
  gated by `QAVOR_REAL_PARSER_TESTS=1`; do not call a missing dependency or
  credential a passing end-to-end test.
- Frontend changes: run `pnpm test:unit` and `pnpm build` from `frontend/`.
- Queue, Redis, MinIO, or database changes: verify the actual configured
  service and clearly separate connection success from read/write readiness.

## Delivery

State what changed, the commands actually run and their outcome, and any
remaining runtime or credential-dependent boundary. Do not claim deployment,
CI, model, OCR, database, or queue success without direct evidence.
