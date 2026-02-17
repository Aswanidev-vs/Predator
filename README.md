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

## Disclaimer

**IMPORTANT LEGAL NOTICE**

This application is provided for educational and personal use only. Users are solely responsible for compliance with all applicable laws and regulations in their jurisdiction. 

### Legal Considerations

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

