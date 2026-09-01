[CmdletBinding()]
param(
    [switch]$RealParser,
    [string]$Python = "python"
)

# Static/package acceptance for the document-ingestion pipeline. The optional
# real-parser pass is intentionally opt-in because it can download Docling and
# OCR models. Service acceptance requires PostgreSQL, Redis, and MinIO, and is
# documented in docs/DEVELOPMENT.md rather than guessed by this local script.
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$Command
    )

    Write-Host "==> $Name"
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

$repositoryRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repositoryRoot
try {
    Invoke-CheckedCommand "race tests" { go test -race ./internal/ingestion ./internal/worker -count=1 }
    Invoke-CheckedCommand "Go test suite" { go test ./... -count=1 }
    Invoke-CheckedCommand "Go vet" { go vet ./... }
    Invoke-CheckedCommand "server build" { go build ./cmd/server }
    Invoke-CheckedCommand "lightweight Python parser tests" {
        & $Python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
    }

    if ($RealParser) {
        $previousRealParserTests = $env:QAVOR_REAL_PARSER_TESTS
        try {
            $env:QAVOR_REAL_PARSER_TESTS = "1"
            Invoke-CheckedCommand "real Python parser tests" {
                & $Python -m unittest discover -s pkg/documentparser/python -p "test_*.py" -v
            }
        }
        finally {
            if ($null -eq $previousRealParserTests) {
                Remove-Item Env:QAVOR_REAL_PARSER_TESTS -ErrorAction SilentlyContinue
            }
            else {
                $env:QAVOR_REAL_PARSER_TESTS = $previousRealParserTests
            }
        }
    }

    Invoke-CheckedCommand "whitespace check" { git diff --check }
}
finally {
    Pop-Location
}
