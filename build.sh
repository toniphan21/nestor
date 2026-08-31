#!/usr/bin/env bash
set -euo pipefail

go mod tidy

echo "build claude/spec"
go build -o ./claude/spec ./cmd/claude-spec

echo "build claude/chat-reader"
go build -o ./claude/chat-reader ./cmd/claude-chat-reader

echo "build claude/output-reader"
go build -o ./claude/output-reader ./cmd/claude-output-reader
