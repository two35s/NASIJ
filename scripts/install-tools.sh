#!/usr/bin/env bash
# install-tools.sh — Install NASIJ developer tooling
set -euo pipefail

echo "Installing golangci-lint..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

echo "Installing goimports..."
go install golang.org/x/tools/cmd/goimports@latest

echo "All tools installed."
