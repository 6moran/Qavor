"""Static contracts for the document-ingestion CI and operator documentation.

These checks deliberately avoid importing parser backends, so they remain part
of the default lightweight Python suite.
"""

from __future__ import annotations

import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]


class DocumentIngestionDocumentationContractTests(unittest.TestCase):
    def test_ci_runs_lightweight_tests_and_gates_real_parser_dependencies(self) -> None:
        workflow = (ROOT / ".github/workflows/ci.yml").read_text(encoding="utf-8")

        self.assertIn("Run lightweight document parser tests", workflow)
        self.assertIn(
            'python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v',
            workflow,
        )
        self.assertIn("constraints.txt", workflow)
        self.assertIn("requirements-test.txt", workflow)
        self.assertIn("run_real_parser_tests", workflow)
        self.assertIn("QAVOR_REAL_PARSER_TESTS:", workflow)

    def test_documentation_states_the_parser_operational_boundaries(self) -> None:
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        architecture = (ROOT / "docs/ARCHITECTURE.md").read_text(encoding="utf-8")
        development = (ROOT / "docs/DEVELOPMENT.md").read_text(encoding="utf-8")
        api = (ROOT / "docs/API.md").read_text(encoding="utf-8")
        combined = "\n".join((readme, architecture, development, api))

        for required in (
            "pool_size",
            "max_tasks_per_worker",
            "Docling",
            "整份 PDF",
            "不设置单文档超时",
            "constraints.txt",
            "QAVOR_REAL_PARSER_TESTS",
            "parser_pool",
        ):
            with self.subTest(required=required):
                self.assertIn(required, combined)

        self.assertIn("轻量", development)
        self.assertIn("真实解析", development)
        self.assertIn("PostgreSQL", development)
        self.assertIn("Redis", development)
        self.assertIn("MinIO", development)
        self.assertIn("隐藏", architecture)
        self.assertIn("关闭", architecture)
        self.assertIn("available_workers", api)

    def test_static_acceptance_script_keeps_real_parser_opt_in(self) -> None:
        script = (ROOT / "scripts/verify_document_ingestion.ps1").read_text(encoding="utf-8")

        self.assertIn("go test -race ./internal/ingestion ./internal/worker -count=1", script)
        self.assertIn("go test ./... -count=1", script)
        self.assertIn("go vet ./...", script)
        self.assertIn("go build ./cmd/server", script)
        self.assertIn("QAVOR_REAL_PARSER_TESTS", script)
        self.assertIn("[switch]$RealParser", script)


if __name__ == "__main__":
    unittest.main()
