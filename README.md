# Predator



A simple desktop app for downloading YouTube videos and audio. Built with Go and Wails, using yt-dlp (go-ytdlp) under the hood.

## What it does

- Download YouTube videos in any resolution from 144p up to 4K
- Extract just the audio in MP3, M4A, Opus, or WAV format
- Shows you file sizes before you download
- Track download progress with speed and ETA
- Cancel downloads if you change your mind
- Pick where you want files saved
- Keeps a history of what you've downloaded

## Getting started

You'll need Go 1.24 or newer installed.

```bash
git clone https://github.com/Aswanidev-vs/Predator.git
cd Predator
go mod download
```

To run it:

```bash
cd Predator
wails dev
```

To build:

```bash
cd Predator
wails build
```

The first time you run it, it'll ask to download yt-dlp and ffmpeg automatically. Just say yes - it handles everything.



## How to use

1. Open the app
2. Click "Change Download Location" to pick where files go (or it'll use the current folder)
3. Paste a YouTube URL
4. Choose video or audio, pick your format
5. Hit download and wait

That's it. The app shows progress in real-time and you can cancel anytime.

## A few things to know

- It tries to download the exact resolution you pick. If that's not available, it falls back to the next best thing.
- Videos get merged into a single MP4 file with H.264 codec for compatibility.
- Audio gets extracted in whatever format you chose.
- Download history is saved locally so you can find your files later.

## Legal stuff

This is for personal use only. Don't download stuff you don't have rights to. Respect copyright and YouTube's terms. I'm not responsible for what you do with this tool.

MIT License - do what you want with the code, just don't blame me if something breaks.
