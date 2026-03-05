# Predator

<p align="center">
<img src="logov4.png" width="200">
</p>

<p align="center">

<a href="https://github.com/Aswanidev-vs/Predator/releases"><img src="https://img.shields.io/github/v/release/Aswanidev-vs/Predator?color=blue&include_prereleases&label=latest" alt="Release"></a>
</p>

A simple desktop app for grabbing videos and audio from YouTube and Instagram. Built with Go and Wails (web tech frontend), using yt-dlp under the hood to do the heavy lifting.

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [How to Use](#how-to-use)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [Technical Details](#technical-details)
- [Requirements](#requirements)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Features

### Video Downloads
- **Multiple resolutions** - From 144p up to 4K (2160p)
- **Best quality option** - Automatically picks the highest available
- **Smart codec handling** - Uses H.264 video + AAC audio for maximum compatibility
- **WebM support** - Available with VP9/AV1 + Opus for better quality

### Audio Downloads
- **Multiple formats** - MP3, M4A, Opus, or WAV
- **High quality output** - Uses best available source and converts to your chosen format
- **MP3**: V0 quality (~245kbps)
- **M4A**: AAC 192kbps
- **WAV**: 16-bit PCM 44.1kHz stereo

### Platform Support
- **YouTube** - Videos, shorts, live streams
- **Instagram** - Reels, posts, stories, TV videos

### User Experience
- **Playlist support** - Download entire playlists or select specific videos
- **File size preview** - See file sizes before downloading
- **Live progress** - Real-time speed, ETA, and percentage tracking
- **Cancel anytime** - Clean cancellation with automatic temp file cleanup
- **Auto-retry** - Failed downloads retry up to 3 times automatically
- **Duplicate detection** - Warns if content was already downloaded
- **Custom download folder** - Set once, remembers forever
- **Download history** - View past downloads, filter by type, open file location
- **Theme toggle** - Dark and light themes (Ctrl+T)

## Getting Started

You'll need **Go 1.24 or newer** installed.

```bash
# Clone the repository
git clone https://github.com/Aswanidev-vs/Predator.git
cd Predator

# Install dependencies
go mod download
```

### Running the App

**Development mode:**
```bash
go run main.go
```

**Build for production:**
```bash
# For Windows
go install fyne.io/tools/cmd/fyne-cross@latest
fyne-cross windows

# For macOS
fyne-cross darwin

# For Linux
fyne-cross linux
```

## Usage

## How to Use

## Architecture

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+L` | Focus the URL input |
| `Ctrl+D` | Start download (when button is active) |
| `Ctrl+T` | Toggle dark/light theme |
| `Esc` | Close any open modal |

**Pro tips:**
- Playlists show a modal where you can select specific videos to download
- Click the folder icon in history to open where a file is saved
- The app auto-updates yt-dlp to the latest version on each download

## Technical Details

### Video Format
- **Container:** MP4 (with WebM option)
- **Video codec:** H.264 (avc1) for maximum device compatibility
- **Audio codec:** AAC (m4a) for best audio quality
- **Fallback handling:** If preferred codec isn't available, automatically falls back to next best option

### Audio Format
- Uses best available source (webm/opus or m4a/aac) and converts to your chosen format using ffmpeg

### File Locations
- **Downloads:** User-selected folder or `PREDATOR_OUTPUT_DIR` environment variable
- **History:** `~/.predator/history.json` (keeps last 100 items)
- **Settings:** `~/.predator/settings.json`
- **Dependencies:** `~/.cache/yt-dlp/` (yt-dlp, ffmpeg, ffprobe)

### Performance
- **Concurrent downloads:** Max 3 at a time to not overwhelm your connection
- **Progress updates:** Every 200ms for smooth progress bars
- **Speed smoothing:** Uses exponential moving average for stable speed display

### Cleanup
- Cancelled downloads automatically clean up `.part` and temp files
- Failed downloads after max retries also trigger cleanup

- `main()`: Initializes the UI and event handlers
- `fetchVideoInfo()`: Dynamically fetches video metadata and available resolutions
- `formatBytes()`: Converts byte sizes to human-readable format
- `formatETA()`: Converts duration to HH:MM:SS format

## Project Structure

```
Predator/
├── main.go              # Main application code
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── logov4.png          # Application icon
├── asset/              # Icon and logo assets
├── fyne-cross/         # Cross-platform build configuration
├── sample.txt          # Code sample reference
└── samplev*.txt        # Additional code samples
```

## Dependencies

- **fyne.io/fyne/v2**: Cross-platform GUI framework
- **github.com/lrstanley/go-ytdlp**: Go wrapper for yt-dlp


## Performance

- **Lazy Metadata Loading**: Video info is only fetched when a valid URL is entered
- **Debounced Input**: 600ms debounce on URL input to prevent excessive API calls
- **Atomic Operations**: Thread-safe state management for concurrent operations
- **Streaming Download**: Supports live progress updates during download

## Disclaimer

**IMPORTANT LEGAL NOTICE**

- Go 1.24+
- Internet connection
- yt-dlp and ffmpeg (auto-installed on first run)

## Troubleshooting

### "ffmpeg not found" error
The app will prompt you to download ffmpeg on first run. If you skip it, you can:
1. Install ffmpeg manually from [ffmpeg.org](https://ffmpeg.org)
2. Or delete `~/.cache/yt-dlp/` and restart the app - it'll ask again

### Slow downloads
- Check your internet connection
- YouTube might be throttling (try a different resolution)
- Max 3 concurrent downloads - close other apps using bandwidth

### Download fails with "format not available"
- Try a lower resolution
- The video might be region-locked or unavailable
- Try using "best" option for auto-selection of best available

### "Permission denied" errors
- Check if your download folder is writable
- Try a different download location

## Contributing

Contributions are welcome! Please read the [CONTRIBUTING.md](CONTRIBUTING.md) file for guidelines.

## Legal Stuff

- **Copyright Compliance**: Users must respect copyright laws and YouTube's Terms of Service. Downloading copyrighted content without proper authorization may be illegal in your jurisdiction.
- **Terms of Service**: By using this application, you acknowledge that you are bound by YouTube's Terms of Service and agree not to violate them.
- **Personal Use Only**: This tool is intended for downloading content you own or have permission to download.
- **Liability**: The authors and contributors of Predator are not responsible for:
  - Any copyright infringement or violations of third-party rights
  - Misuse of downloaded content
  - Any legal consequences arising from use of this application
  - Data loss or corruption
  - Any other damages resulting from use of this software

### Responsible Use

Users should:
- Only download content they have permission to download
- Respect content creators' rights and intellectual property
- Use downloaded content in compliance with local laws
- Not distribute copyrighted content without proper licensing
- Be aware that many YouTube videos are protected by copyright

### No Warranty

This application is provided "as-is" without warranty of any kind, express or implied, including but not limited to the warranties of merchantability, fitness for a particular purpose, or non-infringement.

The authors assume no responsibility for any illegal activities or misuse of this tool. By using Predator, you accept all risks and responsibilities associated with your downloads.

Built with [Wails](https://wails.io/) + Go + vanilla JS. No React, no bloat.

