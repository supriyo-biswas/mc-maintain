#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
output_path="${1:-$repo_root/mc-linux-amd64}"

mkdir -p "$(dirname -- "$output_path")"
cd "$repo_root"

ldflags="$(go run buildscripts/gen-ldflags.go)"
GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -trimpath -tags kqueue --ldflags "$ldflags" -o "$output_path" .

chmod 0755 "$output_path"
echo "Built Linux AMD64 binary at $output_path"
