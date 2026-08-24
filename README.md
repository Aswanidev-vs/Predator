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


**Pro tips:**
- Playlists show a modal where you can select specific videos to download
- Click the folder icon in history to open where a file is saved
- The app auto-updates yt-dlp to the latest version on each download

## Technical Details

### Video Format
- **Container:** MP4 (with WebM option)
- **Video codec:** H.264 (avc1) for maximum device compatibility
- **Audio codec:** AAC (m4a) for best audio quality
- **Codec handling:** When the source is already H.264+AAC (the common case), video and audio are copied into MP4 without re-encoding for speed. If the source uses a codec that can't be muxed into MP4 (VP9, AV1, Opus, etc.), the app re-encodes to H.264+AAC so the merge always succeeds.
- **Fallback handling:** If preferred codec isn't available, automatically falls back to next best option

### Audio Format
- Uses best available source (webm/opus or m4a/aac) and converts to your chosen format using ffmpeg

### File Locations
- **Downloads:** User-selected folder or `PREDATOR_OUTPUT_DIR` environment variable
- **History:** `~/.predator/history.json` (keeps last 100 items)
- **Settings:** `~/.predator/settings.json`
- **Dependencies:** go-ytdlp's cache directory — `%LOCALAPPDATA%/go-ytdlp` on Windows (`~/Library/Caches/go-ytdlp` on macOS, `~/.cache/go-ytdlp` on Linux). yt-dlp and ffmpeg are installed there automatically.

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

### "ffmpeg not found" / video downloads with no audio
Predator auto-locates ffmpeg (from your system `PATH` or its own bundled copy) and passes its location to yt-dlp, so video + audio merging works in almost all cases. If a download still ends up as a raw `.f137.mp4` fragment with no audio:
1. Ensure ffmpeg is on your system `PATH`, or let the app install the bundled copy when it prompts. The app also auto-installs ffmpeg into its cache (`%LOCALAPPDATA%/go-ytdlp`) if none is found.
2. If you declined the first-run install, restart the app to re-trigger it, or install ffmpeg manually from [ffmpeg.org](https://ffmpeg.org).
3. The next download automatically recovers and merges any orphaned fragments left behind by a skipped merge — you usually don't need to re-download.

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

## Changelog

### Unreleased
- **Fixed:** MP4 video downloaded but had no audio / was a raw `.f137.mp4` fragment with the task reported as "success". Root cause: yt-dlp couldn't find ffmpeg in the running process's `PATH`, and because `--ignore-errors` swallows that failure it silently skipped the merge and exited 0. The app now auto-locates ffmpeg (system `PATH` → go-ytdlp cache → auto-installed bundled copy) and passes it to yt-dlp via `--ffmpeg-location`, so the merge always runs.
- **Added:** Merge-recovery safety net — after a "successful" video download, if no merged file exists, the orphaned video/audio fragments are merged directly with ffmpeg, so a fragment-only result is never reported as done.
- **Fixed:** The dependency cache path was mismatched (`~/.cache/yt-dlp` vs go-ytdlp's real `%LOCALAPPDATA%/go-ytdlp`), which meant "Install Dependencies" placed ffmpeg where yt-dlp never looked. Bundled ffmpeg now lands in the correct cache.
- **Fixed:** The title-based file lookup could pick a raw stream fragment as the delivered file; it now skips fragments.

### v1.3.0
- **Fixed:** Audio not merging with video for certain YouTube videos. Old behavior used `--remux-video` which only remuxes the container without re-encoding, causing Opus audio to be silently copied into MP4 without conversion to AAC. Now uses `--recode-video` with explicit FFmpeg transcoding to ensure all downloads produce standard H.264+AAC MP4 files.
- **Fixed:** App would block permanently after 100 downloads due to a dead channel write that was never consumed.
- **Fixed:** Corrupted history file would silently destroy all download history. Now backs up the corrupted file and starts fresh.
- **Fixed:** yt-dlp update failure would panic and crash the entire app instead of returning an error gracefully.
- **Fixed:** Concurrent yt-dlp updates from multiple download workers could race on the binary file. Updates now run once per session.
- **Fixed:** URL validation was completely broken in the frontend — `IsValidYouTubeURL()` returns a Promise but was checked synchronously, so invalid URLs always passed through.
- **Fixed:** Error in playlist URL detection could permanently lock the UI into a "fetching" state, blocking all subsequent URL fetches.
- **Fixed:** Download type radio buttons were invisible to screen readers (`display: none`). Now uses a visually-hidden pattern for accessibility.
- **Fixed:** History was displayed in reverse order (oldest first) instead of newest first.
- **Fixed:** Various `null` pointer risks from unchecked `querySelector(':checked')` calls.

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

