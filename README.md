# Lipi

AI-powered CLI tool for generating and translating subtitles from video and audio files.

## Features

- **AI Transcription** - Generate subtitles from audio/video using Google Gemini or OpenAI Whisper
- **AI Translation** - Translate existing subtitles using Gemini, OpenAI, or Anthropic Claude
- **Multiple Formats** - Support for SRT, VTT, and ASS/SSA subtitle formats
- **Audio Extraction** - Extract audio tracks from video files
- **Parallel Processing** - Concurrent chunk transcription and batch translation for performance
- **Bilingual Mode** - Create overlay subtitles with both translated and original text

## Installation

### Quick Install

**Linux/macOS (Bash):**

```bash
curl -fsSL https://raw.githubusercontent.com/mgpai22/lipi/main/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/mgpai22/lipi/main/install.ps1 | iex
```

**Options:**

```bash
# Install specific version
curl -fsSL https://raw.githubusercontent.com/mgpai22/lipi/main/install.sh | bash -s -- --tag v0.1.0

# Install to current directory
curl -fsSL https://raw.githubusercontent.com/mgpai22/lipi/main/install.sh | bash -s -- --local
```

The release binaries include bundled FFmpeg, so no additional dependencies are required.

### Build from Source

**Prerequisites:**

- Go 1.25 or later

When building from source without the `ffmpeg_embedded` tag, Lipi will automatically
download a prebuilt FFmpeg/FFprobe bundle on first run if it cannot find FFmpeg on
your system. Set `LIPI_FFMPEG_PATH` and `LIPI_FFPROBE_PATH` to point to custom binaries.

```bash
git clone https://github.com/shishir/lipi.git
cd lipi
make build
```

The binary will be available at `./bin/lipi`.

### Build a Fat Binary (Bundled FFmpeg)

To bundle FFmpeg/FFprobe directly into the Lipi binary, run:

```bash
make build-bundled
```

This downloads prebuilt assets into `internal/ffmpeg/assets` (ignored by git)
and embeds them in the resulting binary. The bundled binary will be available
at `./bin/lipi`.

## Quick Start

```bash
# Set your API key
export GEMINI_API_KEY="your-api-key"

# Generate subtitles from a video
lipi generate video.mp4

# Translate subtitles to Japanese
lipi translate video.srt --target-language japanese
```

## Commands

### Generate Subtitles

Generate subtitles from audio or video files using AI transcription.

```bash
lipi generate [media_file] [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | Transcription provider (gemini, openai) | gemini |
| `--model` | Model to use for transcription | gemini-2.5-flash |
| `-f, --format` | Output format (srt, vtt, ass) | srt |
| `-d, --chunk-duration` | Chunk duration in minutes | 1 |
| `--concurrency` | Number of parallel workers | 3 |
| `--transcript-language` | Output language for transcript | native |
| `-k, --api-key` | API key (or use environment variable) | - |
| `-o, --output` | Output file path | auto-generated |

**Examples:**

```bash
# Generate SRT subtitles using Gemini
lipi generate video.mp4

# Generate VTT subtitles using OpenAI Whisper
lipi generate podcast.mp3 --provider openai --format vtt

# Custom chunk size and concurrency
lipi generate movie.mkv --chunk-duration 2 --concurrency 5 -o movie.srt
```

### Translate Subtitles

Translate existing subtitle files to another language.

```bash
lipi translate [subtitle_file] [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `-t, --target-language` | Target language (required) | - |
| `--provider` | Translation provider (gemini, openai, anthropic) | gemini |
| `--model` | Model to use for translation | provider-specific |
| `--overlay` | Create bilingual subtitles | false |
| `--concurrency` | Number of parallel workers | 3 |
| `--batch-size` | Subtitle entries per API request | 50 |
| `-k, --api-key` | API key (or use environment variable) | - |
| `-o, --output` | Output file path | auto-generated |

**Examples:**

```bash
# Translate to Spanish
lipi translate video.srt --target-language spanish

# Bilingual subtitles (translated + original)
lipi translate video.ass --target-language ja --overlay

# Translate using Anthropic Claude
lipi translate video.vtt --provider anthropic --target-language french
```

### Extract Audio

Extract audio from a video file.

```bash
lipi extract [video_file] [flags]
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `-f, --format` | Output format (wav, mp3, aac, flac) | wav |
| `-r, --sample-rate` | Sample rate in Hz | 16000 |
| `-c, --channels` | Number of channels (1=mono, 2=stereo) | 1 |
| `-b, --bitrate` | Bitrate for lossy formats (e.g., 128k) | - |
| `-o, --output` | Output file path | auto-generated |

**Examples:**

```bash
# Extract as WAV (default)
lipi extract video.mp4

# Extract as MP3 with custom settings
lipi extract video.mp4 -f mp3 -r 44100 -c 2 -b 192k
```

### Version

Display version information.

```bash
lipi version
```

## Configuration

### API Keys

Set API keys via environment variables:

```bash
# Google Gemini
export GEMINI_API_KEY="your-gemini-key"

# OpenAI
export OPENAI_API_KEY="your-openai-key"

# Anthropic
export ANTHROPIC_API_KEY="your-anthropic-key"
```

Or pass them directly with the `--api-key` flag.

## Supported Providers & Models

### Transcription

| Provider | Models | Default |
|----------|--------|---------|
| Gemini | gemini-2.5-flash, gemini-2.5-pro, gemini-2.5-flash-lite, gemini-3-flash-preview, gemini-3-pro-preview | gemini-2.5-flash |
| OpenAI | whisper-1 | whisper-1 |

### Translation

| Provider | Models | Default |
|----------|--------|---------|
| Gemini | gemini-2.5-flash, gemini-2.5-pro, gemini-2.5-flash-lite, gemini-3-flash-preview, gemini-3-pro-preview | gemini-2.5-flash |
| OpenAI | gpt-5-mini, gpt-5, gpt-5-nano, gpt-5-pro, o1, o3-mini, o1-pro, o3 | gpt-5-mini |
| Anthropic | claude-haiku-4-5, claude-sonnet-4-5, claude-opus-4-5 | claude-haiku-4-5 |

## Supported Formats

### Media Input

- **Audio:** mp3, wav, aac, flac, ogg, m4a, wma
- **Video:** mp4, mkv, avi, mov, webm, flv, wmv, m4v

### Subtitle Output

- **SRT** - SubRip (most compatible)
- **VTT** - WebVTT (web-friendly)
- **ASS/SSA** - Advanced SubStation Alpha (styling support)

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Format code
make format

# Clean build artifacts
make clean
```

## How It Works

### Transcription Workflow

1. Extract audio from video (if needed)
2. Split audio into chunks (default: 1 minute each)
3. Transcribe chunks in parallel by calling the transcription API
4. Merge segments with adjusted timestamps
5. Generate formatted subtitle file

### Translation Workflow

1. Parse subtitle file (SRT, VTT, or ASS)
2. Extract text entries
3. Batch entries for efficient API usage
4. Translate batches in parallel
5. Optionally overlay with original text
6. Write output preserving format and styling
