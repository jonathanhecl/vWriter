# vWriter — Agent Guide

## What this is

vWriter is a native desktop app (Go + Gio) that turns a creative brief plus
image/video/audio references into a structured full-reference video prompt,
using an Ollama vision model. Single generation mode: Reference.
Everything is in English: UI, errors, logs, docs, comments.

## Requirements

- Go 1.24+
- Ollama reachable at the configured URL (default `http://127.0.0.1:11434`)
  with at least one vision-capable model installed
- ffmpeg + ffprobe available in PATH (video frame sampling, audio metadata)

## Commands

- Run:        `go run .`
- Build:      `go build ./...`
- Test:       `go test ./...`   (ffmpeg-dependent tests skip if ffmpeg is missing)
- Vet/fmt:    `go vet ./...` / `gofmt -l .`
- Dev mode:   set `VWRITER_DEV_MODE=1` → JSONL generation log next to the executable

## Layout

- `main.go` — Gio bootstrap
- `internal/ollama` — Ollama REST client (tags/show/ps/chat-stream/unload)
- `internal/media` — media store, image pipeline, ffmpeg sampling, contact sheets
- `internal/prompt` — vendored guides (go:embed, sha256-checked), system prompts,
  request assembly, context planning, output audit, narrow repair
- `internal/engine` — generate/refine orchestration, phases, cancel, metrics
- `internal/ui`, `internal/app` — Gio UI and app wiring
- `internal/config` — settings persistence (`config.json` next to the executable)
- `guides/` — official guide markdown, embedded; do not edit (sha256-verified)
- `example/` — third-party reference material, not part of the Go build

## Conventions

- Errors carry a stable `Code` (e.g. `CONTEXT_BUDGET_EXCEEDED`) plus a
  user-facing English message; the UI shows code + message + details.
- Keep files under ~300 lines; split by responsibility.
- No new dependencies without justification; prefer the standard library.
- Media is sent only to the configured Ollama URL (local or remote,
  user-chosen); no other network calls.
- Tests: pure packages fully unit-tested; shell out only in `internal/media`
  (skip when ffmpeg is absent).
- Config lives in `config.json` next to the executable (portable-app style);
  never write settings to the OS user-config directory.
