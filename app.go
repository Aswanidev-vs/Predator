package main

import (
	"archive/zip"
	"context"

	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lrstanley/go-ytdlp"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.setupPath()
}

// setupPath adds the cached yt-dlp directory to system PATH
func (a *App) setupPath() {
	cacheDir := getYtdlpCacheDir()
	path := os.Getenv("PATH")
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	if !strings.Contains(path, cacheDir) {
		os.Setenv("PATH", cacheDir+separator+path)
		log.Printf("Added cached tools directory %s to PATH", cacheDir)
	}
}

// ExtractVideoID exposes the private extractVideoID helper to the frontend
func (a *App) ExtractVideoID(url string) string {
	return extractVideoID(url)
}

/* -------------------- Constants -------------------- */

const (
	maxConcurrentDownloads = 3
	progressUpdateInterval = 200 * time.Millisecond
	speedSmoothingAlpha    = 0.2
	cleanupDelay           = 2 * time.Second
	fetchTimeout           = 30 * time.Second
	fetchDebounceDelay     = 600 * time.Millisecond
	maxRetries             = 3
	retryBaseDelay         = 2 * time.Second
	prefOutputDir          = "output_dir"
	mergeTimeout           = 5 * time.Minute // Timeout for merge operations
)

/* -------------------- Types -------------------- */

type DownloadTask struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Resolution   string `json:"resolution"`
	CleanRes     string `json:"cleanRes"`
	AudioFormat  string `json:"audioFormat"`
	AudioQuality string `json:"audioQuality"`
	VideoCodec   string `json:"videoCodec"`
}

type VideoInfo struct {
	Title       string   `json:"title"`
	Duration    float64  `json:"duration"`
	Resolutions []string `json:"resolutions"`
	IsPlaylist  bool     `json:"isPlaylist"`
	PlaylistID  string   `json:"playlistId,omitempty"`
}

type PlaylistVideo struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	URL      string  `json:"url"`
}

type PlaylistInfo struct {
	Title      string          `json:"title"`
	ID         string          `json:"id"`
	VideoCount int             `json:"videoCount"`
	Videos     []PlaylistVideo `json:"videos"`
}

type ProgressUpdate struct {
	TaskID  string  `json:"taskId"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"`
	Speed   string  `json:"speed"`
	ETA     string  `json:"eta"`
	Error   string  `json:"error,omitempty"`
}

type DownloadHistory struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Resolution   string    `json:"resolution"`
	AudioFormat  string    `json:"audioFormat"`
	FilePath     string    `json:"filePath"`
	FileSize     int64     `json:"fileSize"`
	DownloadedAt time.Time `json:"downloadedAt"`
	Status       string    `json:"status"`
}

// Settings represents the application settings
type Settings struct {
	Theme      string `json:"theme"`
	OutputDir  string `json:"outputDir"`
	AutoUpdate bool   `json:"autoUpdate"`
}

/* -------------------- URL Validation -------------------- */

var youtubeURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?://)?(www\.)?(youtube\.com|youtu\.be)/.+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/watch\?v=[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtu\.be/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/shorts/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/live/[\w-]+`),
}

var instagramURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?://)?(www\.)?instagram\.com/p/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?instagram\.com/reel/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?instagram\.com/reels/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?instagram\.com/tv/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?instagram\.com/stories/[\w-]+`),
}

func (a *App) IsValidYouTubeURL(url string) bool {
	for _, pattern := range youtubeURLPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

func (a *App) IsValidInstagramURL(url string) bool {
	for _, pattern := range instagramURLPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

/* -------------------- X / Twitter URL Validation -------------------- */

var xURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?://)?(www\.)?x\.com/[A-Za-z0-9_]+/status/[\d]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?twitter\.com/[A-Za-z0-9_]+/status/[\d]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?x\.com/i/web/status/[\d]+`),
}

/* -------------------- M3U8 URL Validation -------------------- */

// M3U8URLPattern validates direct m3u8 (HLS streaming) URLs
var m3u8URLPattern = regexp.MustCompile(`^(https?://)?.+\.m3u8(\?.*)?$`)

func (a *App) IsValidXURL(url string) bool {
	for _, pattern := range xURLPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

// IsM3U8URL checks if URL is a direct m3u8 HLS stream URL
func (a *App) IsM3U8URL(url string) bool {
	return m3u8URLPattern.MatchString(url)
}

// IsValidURL checks if URL is valid for YouTube, Instagram, X/Twitter, or m3u8 HLS streams
func (a *App) IsValidURL(url string) bool {
	return a.IsValidYouTubeURL(url) || a.IsValidInstagramURL(url) || a.IsValidXURL(url) || a.IsM3U8URL(url)
}

// IsPlaylistURL checks if the URL is a YouTube playlist
func (a *App) IsPlaylistURL(url string) bool {
	return rePlaylistPattern.MatchString(url)
}

// IsInstagramURL checks if the URL is an Instagram URL
func (a *App) IsInstagramURL(url string) bool {
	return a.IsValidInstagramURL(url)
}

// FetchPlaylistInfo fetches all videos from a playlist
func (a *App) FetchPlaylistInfo(url string) (*PlaylistInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout*2) // Longer timeout for playlists
	defer cancel()

	log.Printf("Fetching playlist info for URL: %s", url)

	// Use yt-dlp to get playlist info with flat playlist
	// This outputs the playlist info as first JSON, then each video as separate JSON lines
	result, err := ytdlp.New().
		DumpJSON().
		FlatPlaylist().
		Run(ctx, url)
	if err != nil {
		log.Printf("Error fetching playlist: %v", err)
		return nil, fmt.Errorf("failed to fetch playlist: %w", err)
	}

	log.Printf("Playlist response length: %d", len(result.Stdout))

	// Parse each line as a separate JSON object
	lines := strings.Split(result.Stdout, "\n")

	var playlistTitle, playlistID string
	videos := []PlaylistVideo{}

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// First line should be playlist metadata
		if i == 0 {
			var playlistMeta struct {
				Title   string `json:"title"`
				ID      string `json:"id"`
				Entries []struct {
					ID       string  `json:"id"`
					Title    string  `json:"title"`
					Duration float64 `json:"duration"`
				} `json:"entries"`
			}

			if err := json.Unmarshal([]byte(line), &playlistMeta); err != nil {
				log.Printf("Error parsing playlist metadata: %v", err)
				// Try to continue anyway - might be a different format
				continue
			}

			playlistTitle = playlistMeta.Title
			playlistID = playlistMeta.ID

			// If entries are included in the first response, use them
			for _, entry := range playlistMeta.Entries {
				if entry.ID == "" {
					continue
				}
				videos = append(videos, PlaylistVideo{
					ID:       entry.ID,
					Title:    entry.Title,
					Duration: entry.Duration,
					URL:      fmt.Sprintf("https://youtube.com/watch?v=%s", entry.ID),
				})
			}

			log.Printf("Playlist metadata: ID=%s, Title=%s, Entries=%d", playlistID, playlistTitle, len(playlistMeta.Entries))
			continue
		}

		// Subsequent lines are individual videos (flat playlist format)
		var entry struct {
			ID            string  `json:"id"`
			Title         string  `json:"title"`
			Duration      float64 `json:"duration"`
			PlaylistID    string  `json:"playlist_id"`
			PlaylistTitle string  `json:"playlist_title"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("Error parsing video entry line %d: %v", i, err)
			continue // Skip invalid entries but continue processing
		}

		// Extract playlist info from video entry if not already set
		if playlistID == "" && entry.PlaylistID != "" {
			playlistID = entry.PlaylistID
		}
		if playlistTitle == "" && entry.PlaylistTitle != "" {
			playlistTitle = entry.PlaylistTitle
		}

		if entry.ID == "" {
			continue
		}

		videos = append(videos, PlaylistVideo{
			ID:       entry.ID,
			Title:    entry.Title,
			Duration: entry.Duration,
			URL:      fmt.Sprintf("https://youtube.com/watch?v=%s", entry.ID),
		})
	}

	if playlistID == "" {
		return nil, fmt.Errorf("no playlist found at URL")
	}

	log.Printf("Returning playlist with %d videos", len(videos))

	return &PlaylistInfo{
		Title:      playlistTitle,
		ID:         playlistID,
		VideoCount: len(videos),
		Videos:     videos,
	}, nil
}

// DownloadPlaylist downloads multiple videos from a playlist
func (a *App) DownloadPlaylist(videos []PlaylistVideo, downloadType string, resolution string, audioFormat string) ([]string, error) {
	taskIDs := make([]string, 0, len(videos))

	for _, video := range videos {
		// Fetch video info to get resolutions
		videoInfo, err := a.FetchVideoInfo(video.URL)
		if err != nil {
			log.Printf("Failed to fetch info for %s: %v", video.Title, err)
			continue
		}

		// Determine resolution/audio format
		cleanRes := ""
		if downloadType == "Video" {
			cleanRes = extractResolution(resolution)
		}

		task := DownloadTask{
			URL:          video.URL,
			Title:        videoInfo.Title,
			Type:         downloadType,
			Resolution:   resolution,
			CleanRes:     cleanRes,
			AudioFormat:  audioFormat,
			AudioQuality: "0",
			VideoCodec:   "",
		}

		taskID, err := a.AddToQueue(task)
		if err != nil {
			log.Printf("Failed to add %s to queue: %v", video.Title, err)
			continue
		}

		taskIDs = append(taskIDs, taskID)
	}

	return taskIDs, nil
}

/* -------------------- Helpers -------------------- */

func formatBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div, exp := float64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", b/div, "KMGT"[exp])
}

func formatETA(d time.Duration) string {
	s := int64(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

func formatSpeed(speed float64) string {
	return formatBytes(speed) + "/s"
}

func parseSizeString(sizeStr string) int64 {
	if sizeStr == "" || sizeStr == "Unknown" {
		return 0
	}
	sizeStr = strings.TrimPrefix(sizeStr, "~")
	matches := reParseSize.FindStringSubmatch(sizeStr)
	if len(matches) < 3 {
		return 0
	}
	val, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	multiplier := float64(1024)
	switch matches[2] {
	case "K":
		multiplier = 1024
	case "M":
		multiplier = 1024 * 1024
	case "G":
		multiplier = 1024 * 1024 * 1024
	case "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		multiplier = 1
	}
	return int64(val * multiplier)
}

func truncateError(err string, maxLen int) string {
	if len(err) <= maxLen {
		return err
	}
	return err[:maxLen] + "..."
}

func extractResolution(resolution string) string {
	matches := reExtractRes.FindStringSubmatch(resolution)
	if len(matches) > 1 {
		return matches[1]
	}
	if strings.HasPrefix(resolution, "best") {
		return "best"
	}
	return ""
}

/* -------------------- Format Builders -------------------- */

// buildVideoFormatString builds format string for mp4 container with proper codec handling
// Uses separate video+audio selection to ensure reliable merging
func buildVideoFormatString(cleanRes string, preferH264 bool) string {
	// Build format string that prioritizes h264+aac for mp4 container compatibility
	// Use separate selection (bv*+ba*) to ensure proper download and merging
	// IMPORTANT: Always prefer m4a/aac audio to avoid re-encoding during merge

	if cleanRes == "best" {
		// Best quality - prioritize h264 with aac/m4a for reliable mp4 merging
		// Order: h264+m4a (no re-encode needed) > h264+mp4a > h264+any > vp9+m4a > vp9+any > av01+m4a > av01+any > any+m4a > any+any
		return "bestvideo[vcodec^=avc1]+bestaudio[ext=m4a]/bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=avc1]+bestaudio/" +
			"bestvideo[vcodec^=vp9]+bestaudio[ext=m4a]/bestvideo[vcodec^=vp9]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=vp9]+bestaudio/" +
			"bestvideo[vcodec^=av01]+bestaudio[ext=m4a]/bestvideo[vcodec^=av01]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=av01]+bestaudio/" +
			"bestvideo+bestaudio[ext=m4a]/bestvideo+bestaudio[acodec^=mp4a]/bestvideo+bestaudio/best"
	}

	resNum, _ := strconv.Atoi(cleanRes)
	if resNum >= 1440 {
		// High resolutions (1440p+) - prioritize h264 with exact or close height match
		return fmt.Sprintf(
			// h264 with m4a audio - best for mp4 merging
			"bestvideo[vcodec^=avc1][height=%s]+bestaudio[ext=m4a]/"+
				"bestvideo[vcodec^=avc1][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=avc1][height=%s]+bestaudio/"+
				// h264 with <= height fallback
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[ext=m4a]/"+
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
				// vp9 fallback
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio[ext=m4a]/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
				// av1 fallback
				"bestvideo[vcodec^=av01][height=%s]+bestaudio[ext=m4a]/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
				// Any video codec fallback
				"bestvideo[height<=%s]+bestaudio[ext=m4a]/"+
				"bestvideo[height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[height<=%s]+bestaudio/"+
				"best[height<=%s]",
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
			cleanRes, cleanRes, cleanRes, cleanRes,
		)
	}

	// Standard resolutions - prioritize h264 with m4a audio
	return fmt.Sprintf(
		// h264 with m4a audio - best for mp4 merging
		"bestvideo[vcodec^=avc1][height=%s]+bestaudio[ext=m4a]/"+
			"bestvideo[vcodec^=avc1][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=avc1][height=%s]+bestaudio/"+
			// h264 with <= height fallback
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[ext=m4a]/"+
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
			// vp9 fallback
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio[ext=m4a]/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
			// av1 fallback
			"bestvideo[vcodec^=av01][height=%s]+bestaudio[ext=m4a]/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
			// Any video codec fallback
			"bestvideo[height<=%s]+bestaudio[ext=m4a]/"+
			"bestvideo[height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[height<=%s]+bestaudio/"+
			"best[height<=%s]",
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		cleanRes, cleanRes, cleanRes, cleanRes,
	)
}

// buildWebmFormatString builds format string for webm container with VP9/AV1 + opus
// This ensures best quality webm output with modern codecs
func buildWebmFormatString(cleanRes string) string {
	// For webm, we want VP9 or AV1 video with opus audio
	// This provides better quality than older vp8/opus combinations

	if cleanRes == "best" {
		// Best quality webm - prioritize av01 (AV1) then vp9
		return "bestvideo[vcodec^=av01]+bestaudio[acodec^=opus]/" +
			"bestvideo[vcodec^=av01]+bestaudio[ext=webm]/" +
			"bestvideo[vcodec^=av01]+bestaudio/" +
			"bestvideo[vcodec^=vp9]+bestaudio[acodec^=opus]/" +
			"bestvideo[vcodec^=vp9]+bestaudio[ext=webm]/" +
			"bestvideo[vcodec^=vp9]+bestaudio/" +
			"bestvideo[vcodec^=vp8]+bestaudio[acodec^=opus]/" +
			"bestvideo[vcodec^=vp8]+bestaudio/" +
			"bestvideo+bestaudio[acodec^=opus]/bestvideo+bestaudio/best"
	}

	resNum, _ := strconv.Atoi(cleanRes)
	if resNum >= 1440 {
		// High resolutions - prioritize av01 then vp9
		return fmt.Sprintf(
			// AV1 with opus - best quality
			"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=opus]/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio[ext=webm]/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
				// AV1 <= height fallback
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio[acodec^=opus]/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio[ext=webm]/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio/"+
				// VP9 with opus
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=opus]/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio[ext=webm]/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
				// VP9 <= height fallback
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=opus]/"+
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[ext=webm]/"+
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
				// Any codec fallback
				"bestvideo[height<=%s]+bestaudio[acodec^=opus]/"+
				"bestvideo[height<=%s]+bestaudio[ext=webm]/"+
				"bestvideo[height<=%s]+bestaudio/"+
				"best[height<=%s]",
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
			cleanRes, cleanRes, cleanRes, cleanRes,
		)
	}

	// Standard resolutions - prioritize av01 then vp9 with opus
	return fmt.Sprintf(
		// AV1 with opus
		"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=opus]/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio[ext=webm]/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
			// AV1 <= height fallback
			"bestvideo[vcodec^=av01][height<=%s]+bestaudio[acodec^=opus]/"+
			"bestvideo[vcodec^=av01][height<=%s]+bestaudio[ext=webm]/"+
			"bestvideo[vcodec^=av01][height<=%s]+bestaudio/"+
			// VP9 with opus
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=opus]/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio[ext=webm]/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
			// VP9 <= height fallback
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=opus]/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[ext=webm]/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
			// Any codec fallback
			"bestvideo[height<=%s]+bestaudio[acodec^=opus]/"+
			"bestvideo[height<=%s]+bestaudio[ext=webm]/"+
			"bestvideo[height<=%s]+bestaudio/"+
			"best[height<=%s]",
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		cleanRes, cleanRes, cleanRes, cleanRes,
	)
}

func buildAudioFormatString(format string) string {
	// For all audio formats, we want to select the best quality audio stream
	// regardless of source format, then let ffmpeg convert it to the desired output format
	// This ensures consistent output format (mp3, m4a, etc.) regardless of YouTube's source format
	switch format {
	case "m4a":
		// Prefer m4a source, but accept any best audio and convert
		return "bestaudio[ext=m4a]/bestaudio[acodec^=mp4a]/bestaudio/best"
	case "opus":
		// Opus is usually in webm container
		return "bestaudio[ext=webm][acodec^=opus]/bestaudio[acodec^=opus]/bestaudio/best"
	case "mp3":
		// For MP3, select best audio regardless of source format
		// ffmpeg will convert webm/opus or m4a/aac to mp3
		return "bestaudio/best"
	case "wav":
		// For WAV, select best audio regardless of source format
		return "bestaudio/best"
	default:
		return "bestaudio/best"
	}
}

// buildMergerArgsList returns the ffmpeg codec args tried in order when merging
// video+audio into an MP4. The ladder starts with the fastest copy paths and ends
// with a full H.264/AAC re-encode so a valid MP4 is always produced, even from
// VP9/AV1 video sources that MP4 cannot mux by copy.
func buildMergerArgsList(audioFormat string) []string {
	args := []string{}
	if audioFormat == "m4a" || audioFormat == "aac" {
		// m4a/AAC audio can be copied directly, so keep a copy-fast path first.
		args = append(args,
			"-c:v copy -c:a copy",         // Fast path: copy if compatible
			"-c:v copy -c:a aac -b:a 192k", // Re-encode audio only
		)
	} else {
		// Other audio (opus/webm) must be re-encoded; it can't be copied into mp4.
		args = append(args, "-c:v copy -c:a aac -b:a 192k")
	}
	// Final fallback: re-encode everything to H.264/AAC.
	return append(args, "-c:v libx264 -crf 18 -c:a aac -b:a 192k")
}

/* -------------------- Queue System -------------------- */

var (
	sem         chan struct{}
	semOnce     sync.Once
	activeTasks = make(map[string]context.CancelFunc)
	tasksMu     sync.RWMutex
	taskCounter uint64
	historyMu   sync.RWMutex
	ytdlpOnce   sync.Once
)

var (
	rePlaylistPattern   = regexp.MustCompile(`[?&]list=([a-zA-Z0-9_-]+)`)
	reParseSize         = regexp.MustCompile(`([\d.]+)\s*([KMGT]?)i?B`)
	reExtractRes        = regexp.MustCompile(`^(\d+)p`)
	reVideoIDQuery      = regexp.MustCompile(`[?&]v=([a-zA-Z0-9_-]{11})`)
	reVideoIDShort      = regexp.MustCompile(`youtu\.be/([a-zA-Z0-9_-]{11})`)
	reVideoIDShorts     = regexp.MustCompile(`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`)
	reVideoIDInstaPost  = regexp.MustCompile(`^https?://(www\.)?instagram\.com/p/([a-zA-Z0-9_-]+)`)
	reVideoIDInstaReel  = regexp.MustCompile(`^https?://(www\.)?instagram\.com/reel/([a-zA-Z0-9_-]+)`)
	reVideoIDInstaReels = regexp.MustCompile(`^https?://(www\.)?instagram\.com/reels/([a-zA-Z0-9_-]+)`)
)

/* -------------------- History Management -------------------- */

func (a *App) getHistoryFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("WARNING: UserHomeDir failed: %v, using working directory", err)
		homeDir, _ = os.Getwd()
	}
	historyDir := filepath.Join(homeDir, ".predator")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		log.Printf("WARNING: MkdirAll failed for history dir: %v", err)
	}
	return filepath.Join(historyDir, "history.json")
}

func (a *App) getSettingsFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("WARNING: UserHomeDir failed: %v, using working directory", err)
		homeDir, _ = os.Getwd()
	}
	settingsDir := filepath.Join(homeDir, ".predator")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		log.Printf("WARNING: MkdirAll failed for settings dir: %v", err)
	}
	return filepath.Join(settingsDir, "settings.json")
}

// GetSettings returns the current application settings
func (a *App) GetSettings() (*Settings, error) {
	settingsFile := a.getSettingsFilePath()
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default settings if file doesn't exist
			return a.GetDefaultSettings(), nil
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// SaveSettings saves the application settings to file
func (a *App) SaveSettings(settings Settings) error {
	settingsFile := a.getSettingsFilePath()
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsFile, data, 0644)
}

// GetDefaultSettings returns the default application settings
func (a *App) GetDefaultSettings() *Settings {
	return &Settings{
		Theme:      "dark",
		OutputDir:  "",
		AutoUpdate: true,
	}
}

// CheckDuplicate checks if a video has already been downloaded
func (a *App) CheckDuplicate(videoID string) (map[string]interface{}, error) {
	history, err := a.GetDownloadHistory()
	if err != nil {
		return nil, err
	}

	for _, item := range history {
		// Extract video ID from the URL
		itemVideoID := extractVideoID(item.URL)
		if itemVideoID == videoID {
			return map[string]interface{}{
				"isDuplicate":  true,
				"existingItem": item,
			}, nil
		}
	}

	return map[string]interface{}{
		"isDuplicate":  false,
		"existingItem": nil,
	}, nil
}

// ShowNotification displays a system notification
func (a *App) ShowNotification(title, message string) error {
	wailsRuntime.EventsEmit(a.ctx, "notification", map[string]string{
		"title":   title,
		"message": message,
	})
	return nil
}

// GetDownloadHistory returns all download history

func (a *App) GetDownloadHistory() ([]DownloadHistory, error) {
	historyMu.RLock()
	defer historyMu.RUnlock()

	historyFile := a.getHistoryFilePath()
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []DownloadHistory{}, nil
		}
		return nil, err
	}

	var history []DownloadHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// SaveToHistory adds a completed download to history
func (a *App) SaveToHistory(item DownloadHistory) error {
	historyMu.Lock()
	defer historyMu.Unlock()

	historyFile := a.getHistoryFilePath()

	// Read existing history
	var history []DownloadHistory
	data, err := os.ReadFile(historyFile)
	if err == nil {
		if err := json.Unmarshal(data, &history); err != nil {
			log.Printf("WARNING: History file corrupted, backing up and starting fresh: %v", err)
			backupPath := historyFile + ".bak"
			os.WriteFile(backupPath, data, 0600)
			history = []DownloadHistory{}
		}
	}

	// Filter out duplicate entries with the same URL or FilePath
	var cleanHistory []DownloadHistory
	for _, h := range history {
		if h.URL != item.URL && (h.FilePath == "" || h.FilePath != item.FilePath) {
			cleanHistory = append(cleanHistory, h)
		}
	}
	history = cleanHistory

	// Add new item at the beginning
	history = append([]DownloadHistory{item}, history...)

	// Keep only last 100 items
	if len(history) > 100 {
		history = history[:100]
	}

	// Save back to file
	data, err = json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyFile, data, 0644)
}

// OpenFolder opens the file explorer at the specified path
func (a *App) OpenFolder(path string) error {
	log.Printf("OpenFolder called with path: %s", path)

	// If path is empty, return error
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	// Check if the path exists as-is first
	info, err := os.Stat(path)
	if err == nil {
		// Path exists, determine if it's a file or directory
		if info.IsDir() {
			// It's a directory, just open it
			log.Printf("Opening directory: %s", path)
			return a.openDirectory(path)
		}
		// It's a file, open the containing folder and select the file
		log.Printf("Opening file with /select,: %s", path)
		return a.openDirectoryWithSelect(path)
	}

	// Path might be relative or might not exist anymore
	// Try to get absolute path
	absPath, absErr := filepath.Abs(path)
	if absErr != nil {
		log.Printf("Failed to get absolute path: %v", absErr)
		// Try opening the parent directory anyway
		dir := filepath.Dir(path)
		log.Printf("Trying to open parent directory: %s", dir)
		return a.openDirectory(dir)
	}

	log.Printf("Absolute path: %s", absPath)

	// Check if absolute path exists
	info, err = os.Stat(absPath)
	if err != nil {
		log.Printf("Absolute path does not exist: %v", err)
		// File might have been deleted, try to open the directory anyway
		dir := filepath.Dir(absPath)
		log.Printf("Trying to open directory: %s", dir)
		// Check if directory exists
		if _, dirErr := os.Stat(dir); dirErr != nil {
			return fmt.Errorf("neither file nor directory exists: %s (tried: %s)", path, absPath)
		}
		return a.openDirectory(dir)
	}

	if info.IsDir() {
		log.Printf("Opening absolute directory: %s", absPath)
		return a.openDirectory(absPath)
	}

	log.Printf("Opening absolute file with /select,: %s", absPath)
	return a.openDirectoryWithSelect(absPath)
}

// openDirectory opens a directory using the system file explorer
func (a *App) openDirectory(dirPath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		explorerPath := `C:\Windows\explorer.exe`
		cmd = exec.Command(explorerPath, filepath.Clean(dirPath))
	case "darwin":
		cmd = exec.Command("open", filepath.Clean(dirPath))
	default: // linux
		cmd = exec.Command("xdg-open", filepath.Clean(dirPath))
	}

	log.Printf("Executing directory command: %v", cmd)
	return cmd.Start()
}

// openDirectoryWithSelect opens a directory and selects/highlights the specific file
func (a *App) openDirectoryWithSelect(filePath string) error {
	var cmd *exec.Cmd

	// Ensure we have an absolute path for the file
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	switch runtime.GOOS {
	case "windows":
		// /select, opens the folder and highlights the file
		explorerPath := `C:\Windows\explorer.exe`
		cmd = exec.Command(explorerPath, "/select,", absPath)
	case "darwin":
		// -R flag reveals (selects) the file in Finder
		cmd = exec.Command("open", "-R", absPath)
	default: // linux
		// For linux, just open the parent directory
		cmd = exec.Command("xdg-open", filepath.Dir(absPath))
	}

	log.Printf("Executing select command: %v", cmd)
	return cmd.Start()
}

// ClearHistory clears all download history
func (a *App) ClearHistory() error {

	historyMu.Lock()
	defer historyMu.Unlock()

	historyFile := a.getHistoryFilePath()
	return os.Remove(historyFile)
}

func initSemaphore() {
	semOnce.Do(func() {
		sem = make(chan struct{}, maxConcurrentDownloads)
		for i := 0; i < maxConcurrentDownloads; i++ {
			sem <- struct{}{}
		}
	})
}

func (a *App) generateTaskID() string {
	return fmt.Sprintf("task-%d", atomic.AddUint64(&taskCounter, 1))
}

/* -------------------- Exposed Methods -------------------- */

// getYtdlpCacheDir returns the cache directory for yt-dlp
func getYtdlpCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("WARNING: UserHomeDir failed: %v, using working directory", err)
		homeDir, _ = os.Getwd()
	}
	cacheDir := filepath.Join(homeDir, ".cache", "yt-dlp")
	return cacheDir
}

// InstallWithProgress downloads and installs yt-dlp with progress tracking
func (a *App) InstallWithProgress() error {
	cacheDir := getYtdlpCacheDir()
	os.MkdirAll(cacheDir, 0755)

	// Determine download URL based on OS and architecture
	var downloadURL string
	var filename string

	switch runtime.GOOS {
	case "windows":
		downloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
		filename = "yt-dlp.exe"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			downloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
		} else {
			downloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos_legacy"
		}
		filename = "yt-dlp"
	default: // linux
		downloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
		filename = "yt-dlp"
	}

	// For bundled ffmpeg/ffprobe, use yt-dlp's official ffmpeg builds
	ffmpegURL := fmt.Sprintf("https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-%s-%s.zip",
		getFFmpegOS(), getFFmpegArch())

	// Download yt-dlp first
	wailsRuntime.EventsEmit(a.ctx, "installing-deps-progress", 0, "Downloading yt-dlp...")

	err := a.downloadFileWithProgress(downloadURL, filepath.Join(cacheDir, filename), 0, 50)
	if err != nil {
		return fmt.Errorf("failed to download yt-dlp: %w", err)
	}

	// Make yt-dlp executable (non-Windows)
	if runtime.GOOS != "windows" {
		os.Chmod(filepath.Join(cacheDir, filename), 0755)
	}

	// Download ffmpeg bundle
	wailsRuntime.EventsEmit(a.ctx, "installing-deps-progress", 50, "Downloading ffmpeg...")

	tempDir := filepath.Join(cacheDir, "temp")
	os.MkdirAll(tempDir, 0755)
	zipPath := filepath.Join(tempDir, "ffmpeg.zip")

	err = a.downloadFileWithProgress(ffmpegURL, zipPath, 50, 90)
	if err != nil {
		return fmt.Errorf("failed to download ffmpeg: %w", err)
	}

	// Extract ffmpeg
	wailsRuntime.EventsEmit(a.ctx, "installing-deps-progress", 90, "Extracting ffmpeg...")

	err = a.extractFFmpeg(zipPath, cacheDir)
	if err != nil {
		return fmt.Errorf("failed to extract ffmpeg: %w", err)
	}

	// Cleanup
	os.RemoveAll(tempDir)

	wailsRuntime.EventsEmit(a.ctx, "installing-deps-progress", 100, "Installation complete!")

	return nil
}

// getFFmpegOS returns the OS identifier for ffmpeg builds
func getFFmpegOS() string {
	switch runtime.GOOS {
	case "windows":
		return "win64"
	case "darwin":
		return "macos"
	default:
		return "linux"
	}
}

// getFFmpegArch returns the architecture identifier for ffmpeg builds
func getFFmpegArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x64"
}

// downloadFileWithProgress downloads a file with progress tracking
func (a *App) downloadFileWithProgress(url, destPath string, startPercent, endPercent int) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = 1 // Avoid division by zero
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Create progress reader
	progressReader := &ProgressReader{
		Reader:       resp.Body,
		TotalSize:    totalSize,
		StartPercent: startPercent,
		EndPercent:   endPercent,
		OnProgress: func(percent int, downloaded, total int64) {
			wailsRuntime.EventsEmit(a.ctx, "installing-deps-progress", percent, "")
		},
	}

	_, err = io.Copy(out, progressReader)
	return err
}

// ProgressReader wraps an io.Reader to track progress
type ProgressReader struct {
	Reader       io.Reader
	TotalSize    int64
	CurrentSize  int64
	StartPercent int
	EndPercent   int
	OnProgress   func(percent int, downloaded, total int64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.CurrentSize += int64(n)

	// Calculate percentage within the range
	if pr.TotalSize > 0 {
		downloadPercent := float64(pr.CurrentSize) / float64(pr.TotalSize)
		totalRange := pr.EndPercent - pr.StartPercent
		currentPercent := pr.StartPercent + int(downloadPercent*float64(totalRange))

		if currentPercent > pr.EndPercent {
			currentPercent = pr.EndPercent
		}

		pr.OnProgress(currentPercent, pr.CurrentSize, pr.TotalSize)
	}

	return n, err
}

// extractFFmpeg extracts ffmpeg from zip archive
func (a *App) extractFFmpeg(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Look for ffmpeg and ffprobe executables
		if strings.Contains(f.Name, "ffmpeg") || strings.Contains(f.Name, "ffprobe") {
			// Skip directories and non-executable files
			if f.FileInfo().IsDir() {
				continue
			}

			// Check if it's the main executable (not a symlink or doc)
			if !strings.HasSuffix(f.Name, ".exe") && runtime.GOOS == "windows" {
				continue
			}

			// Derive a safe base name from the archive entry and validate it
			baseName := filepath.Base(f.Name)
			if baseName == "" || baseName == "." || baseName == ".." || strings.Contains(baseName, "..") ||
				strings.Contains(baseName, string(os.PathSeparator)) || strings.Contains(baseName, "/") {
				// Skip potentially unsafe or malicious paths
				continue
			}

			rc, err := f.Open()
			if err != nil {
				return err
			}

			destPath := filepath.Join(destDir, baseName)
			out, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				return err
			}

			_, err = io.Copy(out, rc)
			out.Close()
			rc.Close()

			if err != nil {
				return err
			}

			// Make executable on non-Windows
			if runtime.GOOS != "windows" {
				os.Chmod(destPath, 0755)
			}
		}
	}

	return nil
}

// CheckAndInstallDeps checks and installs required dependencies
func (a *App) CheckAndInstallDeps() error {
	// Check if we have ffmpeg and ffprobe in system
	hasSystemTools := false
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		if _, err := exec.LookPath("ffprobe"); err == nil {
			hasSystemTools = true
		}
	}

	// If we have system tools, just update yt-dlp silently in background
	if hasSystemTools {
		go func() {
			ytdlp.MustInstall(context.Background(), nil)
		}()
		return nil
	}

	// Missing ffmpeg/ffprobe - ask user
	result, err := wailsRuntime.MessageDialog(a.ctx, wailsRuntime.MessageDialogOptions{
		Type:          wailsRuntime.QuestionDialog,
		Title:         "Install Required Tools",
		Message:       "Predator requires ffmpeg and ffprobe for merging video+audio and extracting audio.\n\nThey are not detected on your system.\n\nWe can automatically download open-source bundled versions (yt-dlp + ffmpeg + ffprobe) and cache them locally.\n\nDo you want to continue? (Recommended)",
		Buttons:       []string{"Yes, Install", "No"},
		DefaultButton: "Yes, Install",
		CancelButton:  "No",
	})

	if err != nil {
		return err
	}

	if result == "No" {
		return fmt.Errorf("user declined bundled dependency installation")
	}

	// Show progress dialog
	wailsRuntime.EventsEmit(a.ctx, "installing-deps", true)

	// Use progress-aware installation
	err = a.InstallWithProgress()

	wailsRuntime.EventsEmit(a.ctx, "installing-deps", false)

	if err == nil {
		a.setupPath()
	}

	return err
}

// GetOutputDir returns the current output directory from environment or default
func (a *App) GetOutputDir() string {
	outDir := os.Getenv("PREDATOR_OUTPUT_DIR")
	if outDir == "" {
		outDir, _ := os.Getwd()
		return outDir
	}
	return outDir
}

// SelectOutputDir opens a dialog to select output directory
func (a *App) SelectOutputDir() (string, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Download Location",
	})
	if err != nil {
		return "", err
	}
	// Set environment variable for the session
	if dir != "" {
		os.Setenv("PREDATOR_OUTPUT_DIR", dir)
	}
	return dir, nil
}

// FetchVideoInfo fetches video information from URL
func (a *App) FetchVideoInfo(url string) (*VideoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	result, err := ytdlp.New().DumpJSON().Run(ctx, url)
	if err != nil {
		return nil, err
	}

	var info struct {
		Title    string  `json:"title"`
		Duration float64 `json:"duration"`
		Formats  []struct {
			Height         *int   `json:"height"`
			Width          *int   `json:"width"`
			Filesize       *int64 `json:"filesize"`
			FilesizeApprox *int64 `json:"filesize_approx"`
			Vcodec         string `json:"vcodec"`
			Acodec         string `json:"acodec"`
			Ext            string `json:"ext"`
		} `json:"formats"`
	}

	if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
		return nil, err
	}

	if info.Title == "" {
		return nil, fmt.Errorf("no video found at URL")
	}

	// Build resolution map dynamically from available formats
	resMap := make(map[string]string)
	resHeightMap := make(map[int]string)

	for _, f := range info.Formats {
		if f.Vcodec != "none" && f.Height != nil && *f.Height > 0 {
			res := fmt.Sprintf("%dp", *f.Height)
			var size int64 = 0
			if f.Filesize != nil && *f.Filesize > 0 {
				size = *f.Filesize
			} else if f.FilesizeApprox != nil && *f.FilesizeApprox > 0 {
				size = *f.FilesizeApprox
			}

			// Always add the resolution, even if size is unknown
			// Only update if we have a larger size for this resolution
			existingSize := parseSizeString(resMap[res])
			if size > existingSize {
				if size > 0 {
					resMap[res] = formatBytes(float64(size))
					resHeightMap[*f.Height] = formatBytes(float64(size))
				} else {
					// No size info available - mark as unknown
					resMap[res] = "Unknown"
					resHeightMap[*f.Height] = "Unknown"
				}
			}
		}
	}

	// Extract and sort resolution heights
	var heights []int
	for h := range resHeightMap {
		heights = append(heights, h)
	}
	// Sort in ascending order (lowest to highest)
	for i := 0; i < len(heights); i++ {
		for j := i + 1; j < len(heights); j++ {
			if heights[i] > heights[j] {
				heights[i], heights[j] = heights[j], heights[i]
			}
		}
	}

	// Build options from detected resolutions (sorted) + best
	opts := []string{}
	for _, h := range heights {
		res := fmt.Sprintf("%dp", h)
		size := resHeightMap[h]
		opts = append(opts, fmt.Sprintf("%s (%s)", res, size))
	}
	// Add best option at the end
	opts = append(opts, "best (Unknown)")

	return &VideoInfo{
		Title:       info.Title,
		Duration:    info.Duration,
		Resolutions: opts,
	}, nil
}

// AddToQueue adds a download task to the queue
func (a *App) AddToQueue(task DownloadTask) (string, error) {
	taskID := a.generateTaskID()

	go a.worker(task, taskID)

	return taskID, nil
}

// extractVideoID extracts the video/status ID from YouTube, Instagram, or X/Twitter URL
func extractVideoID(url string) string {
	// YouTube
	if matches := reVideoIDQuery.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}
	if matches := reVideoIDShort.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}
	if matches := reVideoIDShorts.FindStringSubmatch(url); len(matches) > 1 {
		return matches[1]
	}

	// Instagram
	if matches := reVideoIDInstaPost.FindStringSubmatch(url); len(matches) > 2 {
		return "ig_" + matches[2]
	}
	if matches := reVideoIDInstaReel.FindStringSubmatch(url); len(matches) > 2 {
		return "ig_" + matches[2]
	}
	if matches := reVideoIDInstaReels.FindStringSubmatch(url); len(matches) > 2 {
		return "ig_" + matches[2]
	}

	// X/Twitter status ID
	reXStatus1 := regexp.MustCompile(`^(https?://)?(www\.)?x\.com/[A-Za-z0-9_]+/status/([\d]+)`)
	if matches := reXStatus1.FindStringSubmatch(url); len(matches) > 2 {
		return "x_" + matches[2]
	}
	reXStatus2 := regexp.MustCompile(`^(https?://)?(www\.)?twitter\.com/[A-Za-z0-9_]+/status/([\d]+)`)
	if matches := reXStatus2.FindStringSubmatch(url); len(matches) > 2 {
		return "x_" + matches[2]
	}
	reXStatus3 := regexp.MustCompile(`^(https?://)?(www\.)?x\.com/i/web/status/([\d]+)`)
	if matches := reXStatus3.FindStringSubmatch(url); len(matches) > 1 {
		return "x_" + matches[1]
	}

	return ""
}

// CancelTask cancels a running download task and cleans up partial files
func (a *App) CancelTask(taskID string, url string, title string) {
	tasksMu.Lock()
	if cancel, ok := activeTasks[taskID]; ok {
		cancel()
		delete(activeTasks, taskID)
	}
	tasksMu.Unlock()

	// Wait a moment for the worker to stop and release file handles
	time.Sleep(500 * time.Millisecond)

	// Clean up partial files
	outDir := os.Getenv("PREDATOR_OUTPUT_DIR")
	if outDir == "" {
		outDir, _ = os.Getwd()
	}

	// Extract video ID for better matching
	videoID := extractVideoID(url)
	log.Printf("CancelTask: videoID=%s, title=%s, outDir=%s", videoID, title, outDir)

	a.cleanupPartialFiles(outDir, title, videoID)
}

// cleanupPartialFiles removes .part files created by yt-dlp for a given video title
func (a *App) cleanupPartialFiles(outDir, title, videoID string) {
	if outDir == "" || title == "" {
		log.Printf("cleanupPartialFiles: empty outDir or title")
		return
	}

	// Get absolute path for better logging
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		absOutDir = outDir
	}
	log.Printf("Cleaning up partial files for title: %q videoID: %s in dir: %s", title, videoID, absOutDir)

	// Read all files in the directory
	entries, err := os.ReadDir(outDir)
	if err != nil {
		log.Printf("Error reading directory %s: %v", outDir, err)
		return
	}

	// Log all temp files found for debugging
	tempFiles := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".ytdl") || strings.HasSuffix(name, ".temp") {
				tempFiles = append(tempFiles, name)
			}
		}
	}
	log.Printf("Found %d temp files in directory: %v", len(tempFiles), tempFiles)

	// Extract significant words from the title for matching
	titleLower := strings.ToLower(title)
	// Replace special chars with spaces
	for _, char := range []string{"•", "|", "[", "]", "(", ")", "-", "_", ".", ",", "!", "?", ":", ";", "\"", "'"} {
		titleLower = strings.ReplaceAll(titleLower, char, " ")
	}
	// Get words longer than 3 characters
	titleWords := []string{}
	for _, word := range strings.Fields(titleLower) {
		if len(word) > 3 {
			titleWords = append(titleWords, word)
		}
	}
	log.Printf("Title words for matching: %v", titleWords)

	deletedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		filenameLower := strings.ToLower(filename)

		// Check if it's a temp file
		isPartFile := strings.HasSuffix(filenameLower, ".part") ||
			strings.Contains(filenameLower, ".part.") ||
			strings.HasSuffix(filenameLower, ".ytdl") ||
			strings.HasSuffix(filenameLower, ".temp") ||
			(strings.Contains(filenameLower, ".f") && strings.Contains(filenameLower, ".part"))

		if !isPartFile {
			continue
		}

		// Check if filename matches - use video ID if available, otherwise use title words
		matches := false

		// Method 1: Check if video ID is in filename (most reliable - avoids false positives)
		if videoID != "" && strings.Contains(filename, videoID) {
			matches = true
			log.Printf("Match found: videoID %q in filename %q", videoID, filename)
		}

		// Method 2: Check if any significant word from title is in filename
		// Only match if at least 2 words match to reduce false positives from concurrent downloads
		if !matches && len(titleWords) >= 2 {
			matchCount := 0
			for _, word := range titleWords {
				if strings.Contains(filenameLower, word) {
					matchCount++
				}
			}
			// Require at least 2 word matches to reduce false positives
			if matchCount >= 2 {
				matches = true
				log.Printf("Match found: %d title words in filename %q", matchCount, filename)
			}
		}

		// Method 3: Check if file was recently modified AND matches title words (safer than just recent)
		// Removed: purely time-based matching is too risky with concurrent downloads

		// Method 4: If we have very few temp files (less than 3), assume they might be ours
		// Reduced threshold from 5 to 3 to be more conservative
		if !matches && len(tempFiles) <= 3 {
			matches = true
			log.Printf("Match found: low temp file count (%d), assuming %s is ours", len(tempFiles), filename)
		}

		if matches {
			fullPath := filepath.Join(outDir, filename)
			log.Printf("Attempting to delete: %s", fullPath)

			// Try multiple times with delay (file might be locked briefly)
			var removeErr error
			for attempt := 0; attempt < 5; attempt++ {
				if attempt > 0 {
					log.Printf("Retry %d for deleting %s...", attempt, filename)
					time.Sleep(1 * time.Second)
				}
				removeErr = os.Remove(fullPath)
				if removeErr == nil {
					break
				} else {
					log.Printf("Attempt %d failed: %v", attempt, removeErr)
				}
			}

			if removeErr == nil {
				log.Printf("Successfully deleted: %s", filename)
				deletedCount++
			} else {
				log.Printf("Failed to delete %s after 5 attempts: %v", filename, removeErr)
			}
		}
	}

	log.Printf("Cleanup complete. Deleted %d partial files.", deletedCount)

	// Final safety: if we didn't delete anything and there are .part files, log them
	if deletedCount == 0 {
		for _, name := range tempFiles {
			log.Printf("WARNING: Unmatched .part file remains: %s", name)
		}
	}
}

// UpdateYtDlp updates yt-dlp to the latest version
func (a *App) UpdateYtDlp() error {
	var updateErr error
	ytdlpOnce.Do(func() {
		log.Println("Updating yt-dlp to latest version...")
		_, err := ytdlp.Install(context.Background(), nil)
		if err != nil {
			log.Printf("WARNING: yt-dlp update failed: %v", err)
			updateErr = err
		}
	})
	return updateErr
}

/* -------------------- Worker -------------------- */

func (a *App) worker(task DownloadTask, taskID string) {
	initSemaphore()

	<-sem

	ctx, cancel := context.WithCancel(context.Background())
	tasksMu.Lock()
	activeTasks[taskID] = cancel
	tasksMu.Unlock()

	// Get output directory early so it's available in defer
	outDir := os.Getenv("PREDATOR_OUTPUT_DIR")
	if outDir == "" {
		outDir = "."
	}

	defer func() {
		sem <- struct{}{}
		tasksMu.Lock()
		delete(activeTasks, taskID)
		tasksMu.Unlock()

		// If download was cancelled, clean up partial files
		if ctx.Err() == context.Canceled {
			videoID := extractVideoID(task.URL)
			a.cleanupPartialFiles(outDir, task.Title, videoID)
		}

		time.Sleep(cleanupDelay)
	}()

	// Emit task started event
	wailsRuntime.EventsEmit(a.ctx, "task-started", taskID, task)

	var lastDownloaded int
	var lastTime = time.Now()
	var smoothedSpeed float64
	var mu sync.Mutex

	updateProgress := func(p ytdlp.ProgressUpdate) {
		if p.Status != "downloading" {
			return
		}

		mu.Lock()
		now := time.Now()
		elapsed := now.Sub(lastTime).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(p.DownloadedBytes-lastDownloaded) / elapsed
		}
		if smoothedSpeed == 0 {
			smoothedSpeed = speed
		} else {
			smoothedSpeed = speedSmoothingAlpha*speed + (1-speedSmoothingAlpha)*smoothedSpeed
		}
		lastDownloaded = p.DownloadedBytes
		lastTime = now
		currentSpeed := smoothedSpeed
		mu.Unlock()

		update := ProgressUpdate{
			TaskID:  taskID,
			Percent: p.Percent(),
			Status:  "downloading",
			Speed:   formatSpeed(currentSpeed),
			ETA:     formatETA(p.ETA()),
		}

		if p.Percent() >= 100 {
			update.Status = "processing"
			update.Percent = 100
		}

		wailsRuntime.EventsEmit(a.ctx, "task-progress", update)
	}

	// Update yt-dlp before starting
	a.UpdateYtDlp()

	var err error
	var timedOut bool

	// Retry logic
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			wailsRuntime.EventsEmit(a.ctx, "task-retry", taskID, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt) * retryBaseDelay)
		}

		if ctx.Err() == context.Canceled {
			break
		}

		// Detect if this is an m3u8/HLS stream URL
		isM3U8 := a.IsM3U8URL(task.URL)
		// Detect if this is a Twitter/X URL
		isTwitter := strings.Contains(task.URL, "twitter.com") || strings.Contains(task.URL, "x.com")

		// Create timeout context for download + merge operation
		downloadCtx, downloadCancel := context.WithTimeout(ctx, mergeTimeout)
		timedOut = false

		if task.Type == "Video" {
			var format string
			if isM3U8 {
				// For m3u8/HLS streams, use bestvideo with remuxing to mp4
				format = "bestvideo[ext=mp4]/bestvideo[ext=m4a]/bestvideo[ext=webm]/bestvideo/best"
			} else {
				format = buildVideoFormatString(task.CleanRes, true)
			}

			// Build referer arg if Twitter/X
			refererArg := ""
			if isTwitter {
				refererArg = "--referer=https://x.com/"
			}

			mergerArgsList := buildMergerArgsList(task.AudioFormat)

			// Try each merger args until one succeeds
			for i, mergerArgs := range mergerArgsList {
				dl := ytdlp.New().
					Format(format).
					NoPlaylist().
					NoCheckCertificates().
					IgnoreErrors().
					Impersonate("chrome").
					Downloader("aria2c").
					Downloader("m3u8:aria2c").
					DownloaderArgs("aria2c:-x 16 -k 1M "+refererArg).
					MergeOutputFormat("mp4").
					AudioQuality("0").
					PostProcessorArgs("Merger+ffmpeg:" + mergerArgs).
					Output(filepath.Join(outDir, "%(title)s [%(id)s] (%(resolution)s).%(ext)s")).
					ProgressFunc(progressUpdateInterval, updateProgress)

				_, err = dl.Run(downloadCtx, task.URL)

				if err == nil {
					break // Success
				}

				// Check if it's a merge/codec error worth retrying with re-encode
				errStr := err.Error()
				isMergeError := strings.Contains(errStr, "codec") ||
					strings.Contains(errStr, "merge") ||
					strings.Contains(errStr, "Invalid data") ||
					strings.Contains(errStr, "exit status")

				if isMergeError && i < len(mergerArgsList)-1 {
					log.Printf("Merge failed with '%s', retrying with re-encode: %v", mergerArgs, err)
					continue // Try next merger args
				}

				break // Non-merge error or last attempt
			}
			downloadCancel()
		} else if task.Type == "Image" {
			// Download images (or video thumbnails/media) from X/Twitter.
			refererArg := ""
			if isTwitter {
				refererArg = "--referer=https://x.com/"
			}

			dl := ytdlp.New().
				Format("best").
				NoPlaylist().
				NoCheckCertificates().
				IgnoreErrors().
				Impersonate("chrome").
				Downloader("aria2c").
				Downloader("m3u8:aria2c").
				DownloaderArgs("aria2c:-x 16 -k 1M "+refererArg).
				Output(filepath.Join(outDir, "%(title)s [%(id)s].%(ext)s")).
				ProgressFunc(progressUpdateInterval, updateProgress)

			_, err = dl.Run(downloadCtx, task.URL)
			downloadCancel()
		} else {
			// Audio download
			var format string
			if isM3U8 {
				// For m3u8/HLS audio streams
				format = "bestaudio[ext=m4a]/bestaudio[ext=webm]/bestaudio/best"
			} else {
				format = buildAudioFormatString(task.AudioFormat)
			}

			refererArg := ""
			if isTwitter {
				refererArg = "--referer=https://x.com/"
			}

			dl := ytdlp.New().
				ExtractAudio().
				AudioFormat(task.AudioFormat).
				AudioQuality("0").
				NoPlaylist().
				NoCheckCertificates().
				IgnoreErrors().
				Impersonate("chrome").
				Downloader("aria2c").
				Downloader("m3u8:aria2c").
				DownloaderArgs("aria2c:-x 16 -k 1M "+refererArg).
				Format(format).
				Output(filepath.Join(outDir, "%(title)s [%(id)s].%(ext)s")).
				ProgressFunc(progressUpdateInterval, updateProgress)

			// For MP3 and WAV, we need to ensure proper conversion from webm/opus sources
			// Add postprocessor args to force re-encoding to the target format
			// Removed deprecated -strict experimental flag (not needed in modern ffmpeg)
			if task.AudioFormat == "mp3" {
				// Force re-encode to MP3 with high quality VBR (V0 ~ 245kbps)
				// Using -id3v2_version 3 for better compatibility
				dl = dl.PostProcessorArgs("FFmpegExtractAudio:-q:a 0 -id3v2_version 3")
			} else if task.AudioFormat == "wav" {
				// Force re-encode to WAV 16-bit PCM 44.1kHz
				dl = dl.PostProcessorArgs("FFmpegExtractAudio:-acodec pcm_s16le -ar 44100 -ac 2")
			} else if task.AudioFormat == "m4a" {
				// For m4a, ensure we're using AAC codec (no -strict experimental needed)
				dl = dl.PostProcessorArgs("FFmpegExtractAudio:-c:a aac -b:a 192k")
			}

			_, err = dl.Run(downloadCtx, task.URL)
			if downloadCtx.Err() == context.DeadlineExceeded {
				timedOut = true
			}
			downloadCancel()
		}

		if err == nil {
			break
		}

		// Log full details for twitter failures
		if strings.Contains(task.URL, "twitter.com") || strings.Contains(task.URL, "x.com") {
			log.Printf("Twitter/X download attempt failed: %v", err)
		}

		if ctx.Err() == context.Canceled {
			break
		}
	}

	// Emit completion event
	if err != nil {
		if ctx.Err() == context.Canceled {
			wailsRuntime.EventsEmit(a.ctx, "task-cancelled", taskID)
		} else if timedOut {
			// Timeout occurred during download or merge
			wailsRuntime.EventsEmit(a.ctx, "task-error", taskID, "Failed: download/merge timed out. Try lower resolution or check connection.")
		} else {
			errStr := err.Error()
			if strings.Contains(errStr, "codec") ||
				strings.Contains(errStr, "merge") ||
				strings.Contains(errStr, "ffmpeg") ||
				strings.Contains(errStr, "postprocessor") ||
				strings.Contains(errStr, "Invalid data") ||
				strings.Contains(errStr, "No such file") ||
				strings.Contains(errStr, "exit status") {
				wailsRuntime.EventsEmit(a.ctx, "task-error", taskID, "Failed: codec/merge error. Try lower resolution or different format.")
			} else {
				wailsRuntime.EventsEmit(a.ctx, "task-error", taskID, truncateError(err.Error(), 80))
			}
		}
	} else {
		wailsRuntime.EventsEmit(a.ctx, "task-completed", taskID, task)

		// Find the actual downloaded file
		var actualFilePath string
		var fileSize int64

		// Extract video ID for reliable file matching
		videoID := extractVideoID(task.URL)

		// Method 1: Find file by video ID (most reliable - avoids title matching issues)
		if videoID != "" {
			if entries, err := os.ReadDir(outDir); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() && strings.Contains(entry.Name(), videoID) {
						fullPath := filepath.Join(outDir, entry.Name())
						if info, err := os.Stat(fullPath); err == nil {
							actualFilePath = fullPath
							fileSize = info.Size()
							break
						}
					}
				}
			}
		}

		// Method 2: Find file by title pattern (fallback)
		if actualFilePath == "" {
			pattern := filepath.Join(outDir, "*"+strings.ReplaceAll(task.Title, " ", "*")+"*")
			if files, err := filepath.Glob(pattern); err == nil && len(files) > 0 {
				// Find the largest file (in case there are multiple matches)
				var largestFile string
				var largestSize int64
				for _, f := range files {
					if info, err := os.Stat(f); err == nil && !info.IsDir() {
						if info.Size() > largestSize {
							largestSize = info.Size()
							largestFile = f
						}
					}
				}
				if largestFile != "" {
					actualFilePath = largestFile
					fileSize = largestSize
				}
			}
		}

		// Method 3: Find most recent mp4/m4a file (last resort)
		if actualFilePath == "" {
			if entries, err := os.ReadDir(outDir); err == nil {
				var mostRecentFile string
				var mostRecentTime time.Time
				for _, entry := range entries {
					if !entry.IsDir() {
						name := strings.ToLower(entry.Name())
						// Only consider video/audio files, not temp files
						if strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".m4a") ||
							strings.HasSuffix(name, ".webm") || strings.HasSuffix(name, ".mkv") {
							info, err := entry.Info()
							if err == nil {
								if info.ModTime().After(mostRecentTime) {
									mostRecentTime = info.ModTime()
									mostRecentFile = filepath.Join(outDir, entry.Name())
								}
							}
						}
					}
				}
				if mostRecentFile != "" {
					actualFilePath = mostRecentFile
					if info, err := os.Stat(mostRecentFile); err == nil {
						fileSize = info.Size()
					}
				}
			}
		}

		// Save to history with actual file path
		historyItem := DownloadHistory{
			ID:           taskID,
			URL:          task.URL,
			Title:        task.Title,
			Type:         task.Type,
			Resolution:   task.Resolution,
			AudioFormat:  task.AudioFormat,
			FilePath:     actualFilePath,
			FileSize:     fileSize,
			DownloadedAt: time.Now(),
			Status:       "completed",
		}

		a.SaveToHistory(historyItem)
	}
}
