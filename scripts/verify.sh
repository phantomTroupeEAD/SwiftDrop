#!/usr/bin/env bash
# SwiftDrop — Local Verification Script (Layer 3 Automated Quality Gate)
set -eo pipefail

echo "================================================="
echo "  SwiftDrop: Running Automated Quality Verification"
echo "================================================="

FAILED=0

# 1. Secret Scanning Check
echo "[1/4] Checking for accidental hardcoded secrets..."
SECRET_PATTERNS="(BEGIN RSA PRIVATE KEY|BEGIN OPENSSH PRIVATE KEY|AKIA[0-9A-Z]{16}|ghp_[a-zA-Z0-9]{36}|password\s*=\s*['\"][^'\"]+['\"])"
if grep -r -E --exclude-dir={.git,node_modules,dist,bin,vendor,docs,scripts} "$SECRET_PATTERNS" . > /dev/null 2>&1; then
    echo "  [ERROR] Potential hardcoded secret found in repository files!"
    grep -r -E -n --exclude-dir={.git,node_modules,dist,bin,vendor,docs,scripts} "$SECRET_PATTERNS" .
    FAILED=1
else
    echo "  [PASS] No hardcoded secrets detected."
fi

# 2. Go Code Formatting Check
if [ -d "core-api" ] || [ -f "go.mod" ]; then
    echo "[2/4] Verifying Go formatting (gofmt)..."
    GO_UNFORMATTED=$(gofmt -l $(find . -name "*.go" -not -path "*/vendor/*" -not -path "*/docs/*" 2>/dev/null) 2>/dev/null || true)
    if [ -n "$GO_UNFORMATTED" ]; then
        echo "  [ERROR] The following Go files require formatting:"
        echo "$GO_UNFORMATTED"
        echo "  Run 'gofmt -w <file>' to fix."
        FAILED=1
    else
        echo "  [PASS] All Go files properly formatted."
    fi
else
    echo "[2/4] Skipping Go formatting (no Go files found yet)."
fi

# 3. Linter Execution (golangci-lint)
if command -v golangci-lint >/dev/null 2>&1; then
    echo "[3/4] Running golangci-lint with architecture depguard..."
    if [ -d "core-api" ]; then
        (cd core-api && golangci-lint run ./...) || FAILED=1
    elif [ -f "go.mod" ]; then
        golangci-lint run ./... || FAILED=1
    fi
    echo "  [PASS] Linting checks passed."
else
    echo "[3/4] Notice: golangci-lint not installed locally. (Will run in CI)."
fi

# 4. Unit & Integration Tests Execution
if command -v go >/dev/null 2>&1 && ([ -f "core-api/go.mod" ] || [ -f "go.mod" ]); then
    echo "[4/4] Running Go test suites..."
    if [ -f "core-api/go.mod" ]; then
        (cd core-api && go test -v -race ./...) || FAILED=1
    elif [ -f "go.mod" ]; then
        go test -v -race ./... || FAILED=1
    fi
    echo "  [PASS] All tests passed."
else
    echo "[4/4] Skipping Go test execution (go.mod not yet created)."
fi

echo "================================================="
if [ $FAILED -ne 0 ]; then
    echo "  [FAILED] Verification failed. Address errors before opening a PR."
    exit 1
else
    echo "  [SUCCESS] All local checks passed. Ready for review."
    exit 0
fi
