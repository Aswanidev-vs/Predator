# Predator - YouTube Downloader

A modern, cross-platform desktop application for downloading videos and audio from YouTube .
It is built using Go and the Fyne GUI framework, with yt-dlp as the download engine.

The project focuses on simplicity, transparency, and responsive user experience while keeping the implementation clean and maintainable.

## Features

- **Video Download**: Download YouTube videos in various resolutions (144p to 2160p)
- **Audio Extraction**: Extract audio from videos in multiple formats (MP3, M4A, Opus, WAV)
- **Real-time Metadata**: Fetches video information dynamically, showing available resolutions with file sizes
- **Progress Tracking**: Live download progress ETA information
- **Cancellable Downloads**: Stop downloads at any time
- **Custom Output Directory**: Select and manage your download location
- **Cross-platform**: Works on Windows, macOS, and Linux

## Requirements

- **Go**: 1.24.0 or higher
- **FFmpeg**: Required for audio extraction and video merging (automatically handled by yt-dlp)
- **yt-dlp**: Automatically installed on first run

## Installation

### Clone the Repository
```bash
git clone https://github.com/Aswanidev-vs/Predator.git
cd Predator
```

### Install Dependencies
```bash
go mod download
```

### Build and Run
```bash
go run main.go
```

### Build for Distribution
The project includes cross-platform build configuration in the `fyne-cross` directory:

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

1. **Launch the Application**: Run the executable or `go run main.go`
2. **Set Download Location**: Click "Change Download Location" to select where videos will be saved
3. **Paste YouTube URL**: Paste a YouTube video link into the URL field
4. **Select Download Type**:
   - **Video**: Choose a resolution and video will be downloaded with best audio merged
   - **Audio**: Choose an audio format (MP3, M4A, Opus, WAV)
5. **Click Download**: Start the download process
6. **Monitor Progress**: Watch real-time progress, speed, and ETA
7. **Cancel if Needed**: Click Cancel button to stop an ongoing download

## Architecture

### Main Components

- **UI Layer**: Built with [Fyne](https://fyne.io/) - a cross-platform GUI framework
- **Download Engine**: Powered by [go-ytdlp](https://github.com/lrstanley/go-ytdlp) - a Go wrapper for yt-dlp
- **Async Operations**: Uses goroutines for non-blocking UI interactions
- **Progress Tracking**: Real-time progress callbacks with ETA calculations

### Key Functions

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

## License

Check the repository for license information.

## Contributing

Contributions are welcome! Feel free to submit issues and pull requests.



**Note**: This application requires an active internet connection to download from YouTube and relies on the yt-dlp project to function.
