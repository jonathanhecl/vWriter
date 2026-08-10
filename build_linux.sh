#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
output_dir="$project_root/dist"
executable="$output_dir/vWriter"

if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.24 or newer must be available in PATH." >&2
  exit 1
fi

mkdir -p "$output_dir"
cd "$project_root"
echo "Building Linux executable..."
go build -o "$executable" .
echo "Built: $executable"
