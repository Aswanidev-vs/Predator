# Predator

<p align="center">
<img src="logov4.png" width="200">
</p>

<p align="center">
<a href="https://github.com/Aswanidev-vs/Predator/stargazers"><img src="https://img.shields.io/github/stars/Aswanidev-vs/Predator?style=flat&color=green" alt="Stars"></a>
<a href="https://github.com/Aswanidev-vs/Predator/releases"><img src="https://img.shields.io/github/v/release/Aswanidev-vs/Predator?color=blue&include_prereleases&label=latest" alt="Release"></a>
<a href="https://github.com/Aswanidev-vs/Predator/releases"><img src="https://img.shields.io/github/downloads/Aswanidev-vs/Predator/total?color=orange" alt="Downloads"></a>
<a href="https://github.com/Aswanidev-vs/Predator/blob/main/LICENSE"><img src="https://img.shields.io/github/license/Aswanidev-vs/Predator?color=yellow" alt="License"></a>
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
wails dev
```

**Build for production:**
```bash
wails build
```

**First time setup:** The app will ask to download yt-dlp and ffmpeg automatically. Just say yes - it handles everything and caches them locally. No need to install anything else manually.

## How to Use

1. **Open the app** - It'll show the main download tab
2. **Set your download folder** - Click "Change Download Location" (or it'll use the current folder)
3. **Paste a URL** - YouTube or Instagram links both work
4. **Pick your format:**
   - For video: Choose resolution (144p to 4K, or "best")
   - For audio: Pick format (MP3, M4A, Opus, WAV)
5. **Hit download** - Watch the progress bar do its thing

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

## Requirements

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

**This tool is for educational and personal use only.** 

- Respect copyright laws and YouTube's Terms of Service
- Only download content you own or have permission to download
- Don't use this to redistribute copyrighted material
- The authors aren't responsible for how you use this tool

Basically: be cool, don't pirate stuff you shouldn't.

## License

MIT License - see [LICENSE](LICENSE) file.

---

Built with [Wails](https://wails.io/) + Go + vanilla JS. No React, no bloat.

