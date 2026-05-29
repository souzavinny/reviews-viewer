#!/usr/bin/env bash
set -euo pipefail

# Go gate — scoped to our source dirs so the Go tool doesn't descend into
# web/node_modules (a JS dep vendors a stray .go file).
GO_PKGS=(./api/... ./cmd/... ./internal/...)
test -z "$(gofmt -l ./cmd ./internal ./api)" || { echo "gofmt needed on:"; gofmt -l ./cmd ./internal ./api; exit 1; }
go vet "${GO_PKGS[@]}"
go build "${GO_PKGS[@]}"
go test "${GO_PKGS[@]}" -race -count=1

# Web gate — Biome + tsc + vite build.
( cd web && bun run check )
