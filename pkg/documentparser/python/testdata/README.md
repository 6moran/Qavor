# Synthetic document-parser fixtures

All files in this directory are small, synthetic Qavor-owned fixtures released
as CC0. They contain no customer, personal, or third-party document content.

`generate_fixtures.py` creates the text, PDF, image, and Office samples using
fixed metadata. It also normalizes Office ZIP entry timestamps so two runs in
the same constrained environment produce identical SHA-256 hashes.

Generate them after installing the parser and test dependencies:

```powershell
& .\.tmp\document-parser-venv\Scripts\python.exe -m pip install -r pkg/documentparser/python/requirements-test.txt
& .\.tmp\document-parser-venv\Scripts\python.exe pkg/documentparser/python/testdata/generate_fixtures.py
Get-FileHash pkg/documentparser/python/testdata/digital.pdf,pkg/documentparser/python/testdata/scanned.pdf,pkg/documentparser/python/testdata/with-image.docx,pkg/documentparser/python/testdata/slides.pptx,pkg/documentparser/python/testdata/table.xlsx,pkg/documentparser/python/testdata/text-image.png -Algorithm SHA256
```

The manifest in `expected.json` gives the exact acceptance markers for real
parser integration tests. `plain.txt` and `structured.md` intentionally contain
headers, a footer/page number, a table, an image URL, and a code block for the
Go `DocumentCleaner` preservation coverage.
