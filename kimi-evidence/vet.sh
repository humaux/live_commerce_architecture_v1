#!/usr/bin/env bash
set -o pipefail
cd "$(git rev-parse --show-toplevel)"
export GOTOOLCHAIN=go1.27.1
go vet -tags browser ./tests/foundation/ 2>&1 | head -60
