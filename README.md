# vWriter

A native desktop app that turns a creative brief plus image, video, and audio
references into a structured full-reference video prompt, powered by a local
or remote [Ollama](https://ollama.com) vision model.

vWriter follows the official MiniMax H3 full-reference prompt-writing guide to
produce a six-section prompt (`subject_definitions`, `summary`,
`retention_analysis`, `detailed_description`, `overall_soundscape`,
`non_diegetic_music`) ready to paste into your video-generation workflow.

## Features

- Full-reference mode: up to 9 pictures, 3 videos, and 3 audio files
  (12 assets total), numbered as `<Picture N>`, `<Video N>`, `<Audio N>`.
- Ordered video contact sheets with Auto, 6-frame, and 8-frame sampling.
- Works with any vision-capable model installed in Ollama.
- Local or remote Ollama server — point the app at any reachable URL.
- Automatic context planning (8K/16K/24K) with manual override.
- Output audit with one automatic narrow-repair pass.
- Editable output, copy, text-only refine, cancel, and keep-loaded control.
- User-editable advanced system prompt.
- Portable: settings live in `config.json` next to the executable.

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

## How it works

```text
Creative brief + local references
                ↓
Official MiniMax H3 guide + Ollama vision model
                ↓
Editable six-section video prompt
                ↓
Copy into your video workflow
```

## Current limitations

- Audio files are preserved as declared `<Audio N>` references, but the model
  does not listen to their signal. Their role must be stated in the brief.
- Video understanding uses the exact ordered contact sheet shown in the
  preview, not the complete encoded video stream.
- vWriter generates text only; it does not run a video model.
- The interface and documentation are in English. Creative briefs may use
  other languages, and user-supplied dialogue and visible text are preserved.

## License

See [LICENSE](LICENSE).
