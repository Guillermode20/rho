#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

output="${RHO_BIN:-./rho}"

echo "Building latest rho -> ${output}"
go build -o "${output}" ./cmd/rho

echo "Running ${output} $*"
exec "${output}" "$@"
