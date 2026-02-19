package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
}

/* -------------------- Constants -------------------- */

const (
	maxConcurrentDownloads = 3
	progressUpdateInterval = 200 * time.Millisecond
	speedSmoothingAlpha    = 0.2
	taskQueueSize          = 100
	cleanupDelay           = 2 * time.Second
	fetchTimeout           = 30 * time.Second
	fetchDebounceDelay     = 600 * time.Millisecond
	maxRetries             = 3
	retryBaseDelay         = 2 * time.Second
	prefOutputDir          = "output_dir"
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
	Duration    int      `json:"duration"`
	Resolutions []string `json:"resolutions"`
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

/* -------------------- URL Validation -------------------- */

var youtubeURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?://)?(www\.)?(youtube\.com|youtu\.be)/.+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/watch\?v=[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtu\.be/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/shorts/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/live/[\w-]+`),
}

func (a *App) IsValidYouTubeURL(url string) bool {
	for _, pattern := range youtubeURLPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
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
	re := regexp.MustCompile(`([\d.]+)\s*([KMGT]?)i?B`)
	matches := re.FindStringSubmatch(sizeStr)
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
	re := regexp.MustCompile(`^(\d+)p`)
	matches := re.FindStringSubmatch(resolution)
	if len(matches) > 1 {
		return matches[1]
	}
	if strings.HasPrefix(resolution, "best") {
		return "best"
	}
	return ""
}

/* -------------------- Format Builders -------------------- */

func buildVideoFormatString(cleanRes string, preferH264 bool) string {
	// Build format string that handles all codecs (h264, vp9, av1)
	// ffmpeg will transcode to h264/aac for mp4 compatibility if needed
	if cleanRes == "best" {
		// Try h264 first, then vp9, then av1, then any best
		return "bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=avc1]+bestaudio/" +
			"bestvideo[vcodec^=vp9]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=vp9]+bestaudio/" +
			"bestvideo[vcodec^=av01]+bestaudio[acodec^=mp4a]/" +
			"bestvideo[vcodec^=av01]+bestaudio/" +
			"bestvideo+bestaudio/best"
	}

	resNum, _ := strconv.Atoi(cleanRes)
	if resNum >= 1440 {
		// High resolutions - try exact height first, then fall back to <=
		// This ensures 4K videos actually download at 4K, not 1080p
		return fmt.Sprintf(
			// Try exact height match first (e.g., exactly 2160p)
			"bestvideo[vcodec^=avc1][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=avc1][height=%s]+bestaudio/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
				// Fall back to <= if exact match not available
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio/"+
				"bestvideo[height<=%s]+bestaudio/"+
				"best[height<=%s]",
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		)
	}

	// Standard resolutions - try exact height first, then fall back
	return fmt.Sprintf(
		// Try exact height match first
		"bestvideo[vcodec^=avc1][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=avc1][height=%s]+bestaudio/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=vp9][height=%s]+bestaudio/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=av01][height=%s]+bestaudio/"+
			// Fall back to <= if exact match not available
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
			"bestvideo[vcodec^=av01][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=av01][height<=%s]+bestaudio/"+
			"bestvideo[height<=%s]+bestaudio/"+
			"best[height<=%s]",
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
	)
}

func buildAudioFormatString(format string) string {
	switch format {
	case "m4a":
		return "bestaudio[ext=m4a]/bestaudio[acodec^=mp4a]/bestaudio/best"
	case "opus":
		return "bestaudio[ext=opus]/bestaudio[acodec^=opus]/bestaudio/best"
	case "mp3":
		return "bestaudio[ext=mp3]/bestaudio/best"
	case "wav":
		return "bestaudio[ext=wav]/bestaudio/best"
	default:
		return "bestaudio/best"
	}
}

/* -------------------- Queue System -------------------- */

var (
	taskQueue   = make(chan DownloadTask, taskQueueSize)
	sem         chan struct{}
	semOnce     sync.Once
	activeTasks = make(map[string]context.CancelFunc)
	tasksMu     sync.RWMutex
	taskCounter uint64
	historyMu   sync.RWMutex
)

/* -------------------- History Management -------------------- */

func (a *App) getHistoryFilePath() string {
	homeDir, _ := os.UserHomeDir()
	historyDir := filepath.Join(homeDir, ".predator")
	os.MkdirAll(historyDir, 0755)
	return filepath.Join(historyDir, "history.json")
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

	// Sort by date descending (newest first)
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
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
		json.Unmarshal(data, &history)
	}

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

	// Clean the path
	path = filepath.Clean(path)
	log.Printf("Cleaned path: %s", path)

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		log.Printf("Path does not exist: %v", err)
		return fmt.Errorf("path does not exist: %s", path)
	}

	log.Printf("Path exists, isDir: %v", info.IsDir())

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Use full path to explorer.exe to avoid PATH issues
		explorerPath := `C:\Windows\explorer.exe`
		// If path is a file, use /select, to highlight it
		// If path is a directory, just open it
		if info.IsDir() {
			log.Printf("Opening directory: %s", path)
			cmd = exec.Command(explorerPath, path)
		} else {
			// /select, opens the folder and highlights the file
			log.Printf("Opening file with /select,: %s", path)
			cmd = exec.Command(explorerPath, "/select,", path)
		}
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}

	log.Printf("Executing command: %v", cmd)
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

// CheckAndInstallDeps checks and installs required dependencies
func (a *App) CheckAndInstallDeps() error {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		if _, err := exec.LookPath("ffprobe"); err == nil {
			ytdlp.MustInstall(context.Background(), nil)
			return nil
		}
	}

	// Ask user for confirmation
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

	_, err = ytdlp.Install(context.Background(), nil)
	wailsRuntime.EventsEmit(a.ctx, "installing-deps", false)

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
		Title    string `json:"title"`
		Duration int    `json:"duration"`
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

	// Build resolution map
	resolutions := []string{"144p", "240p", "360p", "480p", "720p", "1080p", "1440p", "2160p", "best"}
	resMap := make(map[string]string)

	for _, f := range info.Formats {
		if f.Vcodec != "none" && f.Height != nil && *f.Height > 0 {
			res := fmt.Sprintf("%dp", *f.Height)
			var size int64 = 0
			if f.Filesize != nil && *f.Filesize > 0 {
				size = *f.Filesize
			} else if f.FilesizeApprox != nil && *f.FilesizeApprox > 0 {
				size = *f.FilesizeApprox
			}
			if size > 0 {
				existingSize := parseSizeString(resMap[res])
				if size > existingSize {
					resMap[res] = formatBytes(float64(size))
				}
			}
		}
	}

	// Build options
	opts := []string{}
	for _, r := range resolutions {
		size := "Unknown"
		if s, ok := resMap[r]; ok {
			size = s
		}
		opts = append(opts, fmt.Sprintf("%s (%s)", r, size))
	}

	return &VideoInfo{
		Title:       info.Title,
		Duration:    info.Duration,
		Resolutions: opts,
	}, nil
}

// AddToQueue adds a download task to the queue
func (a *App) AddToQueue(task DownloadTask) (string, error) {
	taskID := a.generateTaskID()
	taskQueue <- task

	// Start worker if not already running
	go a.worker(task, taskID)

	return taskID, nil
}

// CancelTask cancels a running download task
func (a *App) CancelTask(taskID string) {
	tasksMu.Lock()
	if cancel, ok := activeTasks[taskID]; ok {
		cancel()
		delete(activeTasks, taskID)
	}
	tasksMu.Unlock()
}

// UpdateYtDlp updates yt-dlp to the latest version
func (a *App) UpdateYtDlp() error {
	log.Println("Updating yt-dlp to latest version...")
	_, err := ytdlp.Install(context.Background(), nil)
	if err != nil {
		ytdlp.MustInstall(context.Background(), nil)
		return err
	}
	return nil
}

/* -------------------- Worker -------------------- */

func (a *App) worker(task DownloadTask, taskID string) {
	initSemaphore()

	<-sem

	ctx, cancel := context.WithCancel(context.Background())
	tasksMu.Lock()
	activeTasks[taskID] = cancel
	tasksMu.Unlock()

	defer func() {
		sem <- struct{}{}
		tasksMu.Lock()
		delete(activeTasks, taskID)
		tasksMu.Unlock()
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

	outDir := os.Getenv("PREDATOR_OUTPUT_DIR")
	if outDir == "" {
		outDir = "."
	}

	var err error

	// Retry logic
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			wailsRuntime.EventsEmit(a.ctx, "task-retry", taskID, attempt+1, maxRetries)
			time.Sleep(time.Duration(attempt) * retryBaseDelay)
		}

		if ctx.Err() == context.Canceled {
			break
		}

		if task.Type == "Video" {
			format := buildVideoFormatString(task.CleanRes, true)
			dl := ytdlp.New().
				Format(format).
				MergeOutputFormat("mp4").
				RemuxVideo("mp4").
				AudioFormat("m4a").
				AudioQuality("0").
				// Use ffmpeg to transcode any codec (vp9, av1, h264) to h264/aac
				// This ensures single mp4 output regardless of source codec
				PostProcessorArgs("FFmpegVideoConvertor:-c:v libx264 -preset fast -crf 23 -c:a aac -b:a 192k").
				Output(filepath.Join(outDir, "%(title)s [%(id)s] (%(resolution)s).%(ext)s")).
				ProgressFunc(progressUpdateInterval, updateProgress)

			_, err = dl.Run(ctx, task.URL)
		} else {
			format := buildAudioFormatString(task.AudioFormat)
			_, err = ytdlp.New().
				ExtractAudio().
				AudioFormat(task.AudioFormat).
				AudioQuality("0").
				Format(format).
				Output(filepath.Join(outDir, "%(title)s [%(id)s].%(ext)s")).
				ProgressFunc(progressUpdateInterval, updateProgress).
				Run(ctx, task.URL)
		}

		if err == nil {
			break
		}

		if ctx.Err() == context.Canceled {
			break
		}
	}

	// Emit completion event
	if err != nil {
		if ctx.Err() == context.Canceled {
			wailsRuntime.EventsEmit(a.ctx, "task-cancelled", taskID)
		} else {
			errStr := err.Error()
			if strings.Contains(errStr, "codec") ||
				strings.Contains(errStr, "merge") ||
				strings.Contains(errStr, "ffmpeg") ||
				strings.Contains(errStr, "postprocessor") {
				wailsRuntime.EventsEmit(a.ctx, "task-error", taskID, "Failed: codec/merge error. Try lower resolution.")
			} else {
				wailsRuntime.EventsEmit(a.ctx, "task-error", taskID, truncateError(err.Error(), 50))
			}
		}
	} else {
		wailsRuntime.EventsEmit(a.ctx, "task-completed", taskID, task)

		// Find the actual downloaded file
		var actualFilePath string
		var fileSize int64

		// Try to find the file with the title pattern
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

		// If still not found, try a broader search
		if actualFilePath == "" {
			// List all files in directory and find the most recent one
			if entries, err := os.ReadDir(outDir); err == nil {
				var mostRecentFile string
				var mostRecentTime time.Time
				for _, entry := range entries {
					if !entry.IsDir() {
						info, err := entry.Info()
						if err == nil {
							if info.ModTime().After(mostRecentTime) {
								mostRecentTime = info.ModTime()
								mostRecentFile = filepath.Join(outDir, entry.Name())
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
