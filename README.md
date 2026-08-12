# vWriter

vWriter is a **MiniMax H3 video prompt studio**. It was built from the start for
[MiniMax H3](https://www.minimax.io/), which is why the output follows MiniMax's
official full-reference prompt format. It is the format H3 itself was trained
to consume, so prompts come out the way the model expects them.

The app turns a creative brief plus image, video, and audio references into a
structured full-reference video prompt, powered by a local or remote
[Ollama](https://ollama.com) vision model.

![vWriter UI](assets/screenshot.png)

vWriter follows the official MiniMax H3 full-reference prompt-writing guide to
produce a six-section prompt (`subject_definitions`, `summary`,
`retention_analysis`, `detailed_description`, `overall_soundscape`,
`non_diegetic_music`) ready to paste into your MiniMax H3 (or any compatible)
video-generation workflow.

## Features

- **Full-reference media support**: up to 9 pictures, 3 videos, and 3 audio files
  (12 assets total), numbered as `<Picture N>`, `<Video N>`, `<Audio N>`.
- **Semantic Asset Types & Roles**: Assign custom types (`person`, `scene`, `clothes`, `accessory`, `reference`, `movement`, `camera`, `music`, `voice`) and labels (e.g. *John*, *office background*) directly to reference assets.
- **First/Last Frame Anchors**: Mark an image as `first_frame` or `last_frame` and the
  output sequence MUST start or end with that exact image.
- **Audio-to-Image Voice Linking**: Link audio voice tracks directly to reference person images.
- **Multi-Part Stories**: Extend a generated prompt into a multi-part story. Each new
  part opens exactly where the previous one ended, using a virtual continuation frame
  (`<Picture N+1>`) with the same reference media re-attached for consistency. Parts can
  be regenerated individually (later parts are dropped, as they depend on the
  regenerated ending) and copied all at once.
- **Prompt Refinement**: Rewrite the current prompt with a plain-language revision
  instruction. Refinement is text-only; reference media is intentionally not re-sent.
- **Interactive Syntax Highlighting**: Colorized view for generated prompts with instant **`Edit` / `Show`** mode toggle.
- **Live Preset & Template Store**: Real-time auto-saving into templates, with automatic `DEFAULT` preset loading on startup.
- **Ordered Video Contact Sheets**: Auto, 6-frame, and 8-frame video sampling modes,
  with re-sampling that rotates through slightly different moments of the same video.
- **Ollama Integration**: Works with any vision-capable model installed on local or remote Ollama servers.
- **Thinking Mode**: Optional thinking output for supporting models, with an automatic
  non-thinking fallback when the thinking budget is exhausted. Models can be kept
  loaded on the server or unloaded after each generation (or on demand).
- **Automatic Context Planning**: Handles context budgets (8K/16K/24K) with manual override capabilities.
- **Output Audit & Repair**: Automatic format compliance check with narrow repair pass.
- **Cancellable Generations**: Interrupt any running generation at any time.
- **Portable Configuration**: Window size (min 1000x800), panel layouts, and settings live in `config.json` next to the executable.

## Requirements

- [Ollama](https://ollama.com) running locally or on a reachable machine,
  with at least one vision-capable model installed (e.g. `ollama pull gemma3`).
- ffmpeg and ffprobe available in `PATH` (used for video frame sampling and
  audio metadata).
- To build from source: Go 1.24 or newer.

## Run from source

```sh
go run .
```

Build a binary for your platform:

```sh
go build -o vWriter .
```

Convenience scripts build native binaries into `dist/`:

```powershell
.\build_and_run_win.ps1
```

```sh
bash build_linux.sh
bash build_macos.sh
```

## Release Script

To create a new release package for Windows 64-bit, macOS Silicon (ARM64), and Linux 64-bit:

```sh
bash release.sh
```

The release script will:
1. Prompt for a release version tag (e.g. `v1.0.0`) and verify it does not already exist.
2. Prompt for your GitHub token (or read `$GITHUB_TOKEN` from environment).
3. Cross-compile binaries for Windows (x64), macOS (ARM64), and Linux (x64).
4. Create release `.zip` archives containing the compiled executables.
5. Ask for confirmation before creating the git tag, pushing to GitHub, and publishing the release assets.

## App Icon and packaging

`appicon.png` is the production app icon. It is generated from the editable
source at `assets/appicon.svg` and is picked up automatically by Gio's
`gogio` packager for macOS and Linux bundles. On Windows, `vwriter_windows_amd64.syso`
embeds the same icon in every executable.

## Developer logging

Set `VWRITER_DEV_MODE=1` to append one JSON record per generation event
(`generate_succeeded`, `extend_succeeded`, `refine_succeeded`) to
`generations.jsonl` next to the executable, with model, timing, token, and
repair metrics.

## License

See [LICENSE](LICENSE).
