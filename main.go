package main

import (
<<<<<<< HEAD
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Predator",
		Width:  520,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			WebviewUserDataPath:  "",
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}
=======
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/lrstanley/go-ytdlp"
)

//go:embed summer.png
var summerBytes []byte

//go:embed dark-mode.png
var darkBytes []byte

func checkAndInstallDeps(w fyne.Window) error {

	// Quick check: do we have system ffmpeg and ffprobe in PATH?
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		if _, err := exec.LookPath("ffprobe"); err == nil {
			// System ffmpeg/ffprobe found → ensure yt-dlp is cached (fast if already there)
			ytdlp.MustInstall(context.Background(), nil)
			return nil
		}
	}
	// First time without system ffmpeg → ask user
	done := make(chan error, 1)

	confirm := dialog.NewConfirm(
		"Install Required Tools",
		"Predator requires ffmpeg and ffprobe for merging video+audio and extracting audio.\n\n"+
			"They are not detected on your system.\n\n"+
			"We can automatically download open-source bundled versions (yt-dlp + ffmpeg + ffprobe) and cache them locally.\n\n"+
			"Do you want to continue? (Recommended)",
		func(ok bool) {
			if !ok {
				done <- fmt.Errorf("user declined bundled dependency installation")
				return
			}

			// Show progress
			bar := widget.NewProgressBarInfinite()
			label := widget.NewLabel("Downloading yt-dlp, ffmpeg & ffprobe…\nThis may take a moment on first run.")
			content := container.NewVBox(label, bar)

			progressDialog := dialog.NewCustomWithoutButtons("Installing Dependencies", content, w)
			progressDialog.Show()
			go func() {
				defer progressDialog.Hide()

				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("installation panicked: %v", r)
					}
				}()

				// Fixed: handle two return values
				_, err := ytdlp.Install(context.Background(), nil)
				if err != nil {
					done <- fmt.Errorf("failed to install dependencies: %w", err)
					return
				}

				done <- nil
			}()

		},
		w,
	)

	confirm.SetDismissText("No")
	confirm.SetConfirmText("Yes, Install")
	confirm.Show()

	return <-done
}

const prefOutputDir = "output_dir"

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
)

/* -------------------- URL Validation -------------------- */

var youtubeURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(https?://)?(www\.)?(youtube\.com|youtu\.be)/.+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/watch\?v=[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtu\.be/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/shorts/[\w-]+`),
	regexp.MustCompile(`^(https?://)?(www\.)?youtube\.com/live/[\w-]+`),
}

func isValidYouTubeURL(url string) bool {
	for _, pattern := range youtubeURLPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

/* -------------------- Helpers -------------------- */

// updateYtDlp explicitly updates yt-dlp to the latest version
func updateYtDlp() error {
	log.Println("Updating yt-dlp to latest version...")
	// Call Install which should update to latest version
	// The second parameter can be used to specify options
	_, err := ytdlp.Install(context.Background(), nil)
	if err != nil {
		// Try with MustInstall as fallback
		log.Println("yt-dlp.Install failed, trying MustInstall:", err)
		ytdlp.MustInstall(context.Background(), nil)
		return err
	}
	log.Println("yt-dlp updated successfully")
	return nil
}

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
	// Remove ~ prefix if present
	sizeStr = strings.TrimPrefix(sizeStr, "~")

	// Parse format like "15.2 MiB" or "1.5 GiB"
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

/* -------------------- Queue System -------------------- */

type DownloadTask struct {
	URL          string
	Title        string
	Type         string
	Resolution   string // Display text like "720p (15.2 MiB)"
	CleanRes     string // Clean resolution like "720" or "best"
	AudioFormat  string
	AudioQuality string
	VideoCodec   string // Preferred video codec
}

var (
	taskQueue = make(chan DownloadTask, taskQueueSize)
	sem       chan struct{}
	semOnce   sync.Once
)

func initSemaphore() {
	semOnce.Do(func() {
		sem = make(chan struct{}, maxConcurrentDownloads)
		for i := 0; i < maxConcurrentDownloads; i++ {
			sem <- struct{}{}
		}
	})
}

func extractResolution(resolution string) string {
	// Extract just the resolution number from strings like "720p (15.2 MiB)"
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

// buildVideoFormatString creates format string with Code 2's superior codec handling
func buildVideoFormatString(cleanRes string, preferH264 bool) string {
	if cleanRes == "best" {
		// For best quality with comprehensive codec fallbacks
		// H.264 + AAC (best compatibility), then VP9, then AV1
		return "bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/" + // H.264 + AAC (best compatibility)
			"bestvideo[vcodec^=avc1]+bestaudio/" + // H.264 + any audio
			"bestvideo[vcodec^=vp9]+bestaudio[acodec^=mp4a]/" + // VP9 + AAC (reencode audio)
			"bestvideo[vcodec^=vp9]+bestaudio/" + // VP9 + any audio
			"bestvideo[vcodec^=av01]+bestaudio[acodec^=mp4a]/" + // AV1 + AAC
			"bestvideo[vcodec^=av01]+bestaudio/" + // AV1 + any audio
			"bestvideo+bestaudio[acodec^=mp4a]/" + // Any video + AAC
			"bestvideo+bestaudio/best" // Ultimate fallback
	}

	// For high resolutions (1440p, 2160p), H.264 is rarely available
	// Prioritize VP9 and AV1 which are commonly used for high-res YouTube videos
	resNum, _ := strconv.Atoi(cleanRes)
	if resNum >= 1440 {
		// High resolution: prioritize VP9/AV1 since H.264 usually not available
		return fmt.Sprintf(
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=av01][height<=%s]+bestaudio/"+
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
				"bestvideo[height<=%s]+bestaudio[acodec^=mp4a]/"+
				"bestvideo[height<=%s]+bestaudio/"+
				"best[height<=%s]",
			cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
		)
	}

	// Standard resolution (1080p and below) with comprehensive codec fallback chains
	// 1. H.264 + AAC (most reliable, always works in MP4)
	// 2. H.264 + any audio (with audio conversion)
	// 3. VP9 + AAC (reencode audio to AAC)
	// 4. VP9 + Opus (may need remux)
	// 5. Any video + AAC audio
	// 6. Final fallback to best available
	return fmt.Sprintf(
		"bestvideo[vcodec^=avc1][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=avc1][height<=%s]+bestaudio/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[vcodec^=vp9][height<=%s]+bestaudio/"+
			"bestvideo[height<=%s]+bestaudio[acodec^=mp4a]/"+
			"bestvideo[height<=%s]+bestaudio/"+
			"best[height<=%s]",
		cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes, cleanRes,
	)
}

func buildAudioFormatString(format string) string {
	// Audio codec priority: aac (m4a) > opus > mp3 > best
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

func worker(downloadsContainer *fyne.Container, outputDir *string) {
	initSemaphore()

	for task := range taskQueue {
		<-sem

		titleLbl := widget.NewLabel(task.Title)
		titleLbl.Wrapping = fyne.TextWrapWord
		titleLbl.TextStyle = fyne.TextStyle{Bold: true}

		progBar := widget.NewProgressBar()
		statLbl := widget.NewLabel("Starting...")
		spdLbl := widget.NewLabel("")
		cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), nil)

		taskCont := container.NewVBox(
			widget.NewSeparator(),
			titleLbl,
			container.NewBorder(nil, nil, nil, cancelBtn, progBar),
			container.NewHBox(statLbl, spdLbl),
		)

		fyne.Do(func() {
			downloadsContainer.Add(taskCont)
			downloadsContainer.Refresh()
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancelBtn.OnTapped = func() {
			cancel()
			cancelBtn.Disable()
			statLbl.SetText("Cancelling...")
		}

		go func(t DownloadTask, c *fyne.Container, ctx context.Context, cancel context.CancelFunc) {
			defer func() {
				sem <- struct{}{}
				time.Sleep(cleanupDelay)
				fyne.Do(func() {
					downloadsContainer.Remove(c)
					downloadsContainer.Refresh()
				})
			}()

			var lastDownloaded int
			var lastTime = time.Now()
			var smoothedSpeed float64
			var mu sync.Mutex // Protect progress variables

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

				fyne.Do(func() {
					if p.Percent() >= 100 {
						progBar.SetValue(1)
						statLbl.SetText("Processing… (merging)")
						spdLbl.SetText("")
						return
					}
					progBar.SetValue(p.Percent() / 100)
					statLbl.SetText(fmt.Sprintf("Downloading… %.1f%%", p.Percent()))
					spdLbl.SetText(fmt.Sprintf("Speed: %s | ETA: %s", formatSpeed(currentSpeed), formatETA(p.ETA())))
				})
			}

			var err error
			outPath := *outputDir

			// Update yt-dlp before starting download
			updateYtDlp()

			// Retry logic with exponential backoff
			for attempt := 0; attempt < maxRetries; attempt++ {
				if attempt > 0 {
					fyne.Do(func() {
						statLbl.SetText(fmt.Sprintf("Retrying... (attempt %d/%d)", attempt+1, maxRetries))
					})
					time.Sleep(time.Duration(attempt) * retryBaseDelay)
				}

				if ctx.Err() == context.Canceled {
					break
				}

				if t.Type == "Video" {
					format := buildVideoFormatString(t.CleanRes, true) // Prefer h264 for compatibility
					dl := ytdlp.New().
						Format(format).
						MergeOutputFormat("mp4").
						NoKeepVideo().
						NoKeepFragments().
						RemuxVideo("mp4").
						// Convert audio to m4a (AAC) for guaranteed MP4 compatibility
						AudioFormat("m4a").
						AudioQuality("0"). // Best quality
						// Postprocessor options to ensure successful merge
						PostProcessorArgs("FFmpegMerger:-c:v copy -c:a aac -b:a 192k").
						Output(filepath.Join(outPath, "%(title)s [%(id)s] (%(resolution)s).%(ext)s")).
						ProgressFunc(progressUpdateInterval, updateProgress)

					_, err = dl.Run(ctx, t.URL)
				} else {
					format := buildAudioFormatString(t.AudioFormat)
					_, err = ytdlp.New().
						ExtractAudio().
						AudioFormat(t.AudioFormat).
						AudioQuality("0"). // Best quality
						Format(format).
						Output(filepath.Join(outPath, "%(title)s [%(id)s].%(ext)s")).
						ProgressFunc(progressUpdateInterval, updateProgress).
						Run(ctx, t.URL)
				}

				if err == nil {
					break // Success
				}

				// Check if error is retryable
				if ctx.Err() == context.Canceled {
					break
				}
			}

			fyne.Do(func() {
				cancelBtn.Disable()
				if err != nil {
					if ctx.Err() == context.Canceled {
						statLbl.SetText("Cancelled")
					} else {
						// Check if it's a codec/merge related error
						errStr := err.Error()
						if strings.Contains(errStr, "codec") ||
							strings.Contains(errStr, "merge") ||
							strings.Contains(errStr, "ffmpeg") ||
							strings.Contains(errStr, "postprocessor") {
							statLbl.SetText("Failed: codec/merge error. Try lower resolution.")
						} else {
							statLbl.SetText("Failed: " + truncateError(err.Error(), 50))
						}
					}
					progBar.SetValue(0)
				} else {
					progBar.SetValue(1)
					statLbl.SetText("Completed ✓")
				}
				spdLbl.SetText("")
			})
		}(task, taskCont, ctx, cancel)
	}
}

/* -------------------- Main -------------------- */

func main() {

	// ytdlp.MustInstall(context.Background(), nil)
	a := app.NewWithID("Predator")
	w := a.NewWindow("Predator")
	w.Resize(fyne.NewSize(520, 480))
	prefs := a.Preferences()
	logo, err := os.ReadFile("logov4.png")
	if err != nil {
		log.Println("Error reading icon file ", err)
	} else {
		appIcon := fyne.NewStaticResource("logov4.png", logo)
		w.SetIcon(appIcon)
	}

	// Load icons
	lightIcon := fyne.NewStaticResource("summer.png", summerBytes)
	darkIcon := fyne.NewStaticResource("dark-mode.png", darkBytes)

	isDark := true

	var themeBtn *widget.Button
	themeBtn = widget.NewButtonWithIcon("", lightIcon, func() {
		if isDark {
			a.Settings().SetTheme(theme.LightTheme())
			themeBtn.SetIcon(darkIcon) // show moon icon
		} else {
			a.Settings().SetTheme(theme.DarkTheme())
			themeBtn.SetIcon(lightIcon) // show sun icon
		}
		isDark = !isDark
	})

	headerLabel := widget.NewLabel("Predator")
	headerContainer := container.NewBorder(nil, nil, headerLabel, themeBtn)

	/* -------------------- UI -------------------- */
	go func() {
		err := checkAndInstallDeps(w)
		if err != nil {
			fyne.Do(func() {
				var msg string
				if strings.Contains(err.Error(), "user declined") {
					msg = "Dependency installation was declined.\n\nVideo downloads and audio extraction will not work properly."
				} else {
					msg = fmt.Sprintf("Failed to install required dependencies (yt-dlp/ffmpeg):\n%s\n\nSome features will be limited.", err)
				}
				dialog.ShowError(fmt.Errorf(msg), w)
			})
		}
	}()

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("Paste YouTube URL here")

	// Create clear button with icon
	clearBtn := widget.NewButtonWithIcon("", theme.ContentClearIcon(), func() {
		urlEntry.SetText("")
	})

	// Create a Border container: Entry in Center (expands), Button on Right
	urlInputContainer := container.NewBorder(nil, nil, nil, clearBtn, urlEntry)

	downloadType := widget.NewRadioGroup([]string{"Video", "Audio"}, nil)
	downloadType.SetSelected("Video")
	downloadType.Horizontal = true

	resolutions := []string{"144p", "240p", "360p", "480p", "720p", "1080p", "1440p", "2160p", "best"}
	resSelect := widget.NewSelect(nil, nil)
	resSelect.Disable()

	audioFormats := []string{"mp3", "m4a", "opus", "wav"}
	audioSelect := widget.NewSelect(audioFormats, nil)
	audioSelect.SetSelected("mp3")
	audioSelect.Disable()

	downloadType.OnChanged = func(s string) {
		if s == "Video" {
			resSelect.Enable()
			audioSelect.Disable()
		} else {
			resSelect.Disable()
			audioSelect.Enable()
		}
	}

	addBtn := widget.NewButton("Add to Queue", nil)
	addBtn.Disable()

	var tabs *container.AppTabs

	titleLabel := widget.NewLabel("")
	titleLabel.Wrapping = fyne.TextWrapWord
	statusLabel := widget.NewLabel("Ready")
	/* -------------------- Output Dir -------------------- */

	outputDir := prefs.String(prefOutputDir)
	outputDirLabel := widget.NewLabel("")

	updateOutputUI := func() {
		if outputDir == "" {
			outputDirLabel.SetText("Download location not set")
		} else {
			outputDirLabel.SetText("Download Location: " + outputDir)
		}
	}

	selectDirectory := func() {
		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputDir = uri.Path()
			prefs.SetString(prefOutputDir, outputDir)
			updateOutputUI()
		}, w).Show()
	}

	if outputDir == "" {
		dialog.ShowInformation("Select Download Location", "Please select a download folder.", w)
		selectDirectory()
	}
	updateOutputUI()

	changeDirBtn := widget.NewButton("Change Download Location", selectDirectory)

	/* -------------------- Dynamic Fetch -------------------- */

	var fetchTimer *time.Timer
	var fetching int32

	urlEntry.OnChanged = func(text string) {
		text = strings.TrimSpace(text)
		if fetchTimer != nil {
			fetchTimer.Stop()
		}
		if text == "" {
			fyne.Do(func() {
				statusLabel.SetText("Ready")
				titleLabel.SetText("")
				resSelect.Disable()
				addBtn.Disable()
			})
			return
		}

		// Validate URL before fetching
		if !isValidYouTubeURL(text) {
			fyne.Do(func() {
				statusLabel.SetText("Invalid YouTube URL")
				titleLabel.SetText("")
				resSelect.Disable()
				addBtn.Disable()
			})
			return
		}

		fetchTimer = time.AfterFunc(fetchDebounceDelay, func() {
			if atomic.LoadInt32(&fetching) == 1 {
				return
			}
			atomic.StoreInt32(&fetching, 1)

			fyne.Do(func() {
				statusLabel.SetText("Fetching video info...")
				resSelect.Disable()
				addBtn.Disable()
			})

			go fetchVideoInfo(text, resolutions, resSelect, statusLabel, titleLabel, addBtn, &fetching)
		})
	}

	/* -------------------- Download -------------------- */

	addBtn.OnTapped = func() {
		url := strings.TrimSpace(urlEntry.Text)
		if url == "" || !isValidYouTubeURL(url) {
			dialog.ShowError(fmt.Errorf("Please enter a valid YouTube URL"), w)
			return
		}

		cleanRes := extractResolution(resSelect.Selected)
		task := DownloadTask{
			URL:         url,
			Title:       strings.TrimPrefix(titleLabel.Text, "Title : "),
			Type:        downloadType.Selected,
			Resolution:  resSelect.Selected,
			CleanRes:    cleanRes,
			AudioFormat: audioSelect.Selected,
		}
		taskQueue <- task

		urlEntry.SetText("")
		titleLabel.SetText("")
		resSelect.Disable()
		addBtn.Disable()
		statusLabel.SetText("Added to queue")
		if tabs != nil {
			tabs.SelectIndex(1)
		}
	}

	downloadsContainer := container.NewVBox()
	downloadsScroll := container.NewVScroll(downloadsContainer)

	go worker(downloadsContainer, &outputDir)

	/* -------------------- Layout -------------------- */

	content := container.NewVBox(
		urlInputContainer,
		titleLabel,
		downloadType,
		container.NewGridWithColumns(2, resSelect, audioSelect),
		widget.NewSeparator(),
		outputDirLabel,
		changeDirBtn,
		addBtn,
		statusLabel,
	)

	tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("New Download", theme.DownloadIcon(), container.NewVScroll(content)),
		container.NewTabItemWithIcon("Queue", theme.ListIcon(), downloadsScroll),
	)

	mainLayout := container.NewBorder(headerContainer, nil, nil, nil, tabs)
	w.SetContent(mainLayout)
	w.ShowAndRun()
}

/* -------------------- Fetch Function -------------------- */

func fetchVideoInfo(
	url string,
	resolutions []string,
	resSelect *widget.Select,
	statusLabel *widget.Label,
	titleLabel *widget.Label,
	downloadBtn *widget.Button,
	fetching *int32,
) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	// Check context before starting
	if ctx.Err() != nil {
		fyne.Do(func() {
			atomic.StoreInt32(fetching, 0)
			statusLabel.SetText("Request timeout")
		})
		return
	}

	result, err := ytdlp.New().DumpJSON().Run(ctx, url)

	fyne.Do(func() {
		defer atomic.StoreInt32(fetching, 0)

		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				statusLabel.SetText("Request timeout - try again")
			} else {
				statusLabel.SetText("Failed to fetch info")
			}
			return
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
			statusLabel.SetText("Failed to parse info")
			return
		}

		if info.Title == "" {
			statusLabel.SetText("No video found at URL")
			return
		}

		titleLabel.SetText("Title : " + info.Title)

		// Build resolution map with better size detection
		resMap := make(map[string]string)
		for _, f := range info.Formats {
			// Only include video formats (has video codec and height)
			if f.Vcodec != "none" && f.Height != nil && *f.Height > 0 {
				res := fmt.Sprintf("%dp", *f.Height)

				// Get the best size estimate available
				var size int64 = 0
				if f.Filesize != nil && *f.Filesize > 0 {
					size = *f.Filesize
				} else if f.FilesizeApprox != nil && *f.FilesizeApprox > 0 {
					size = *f.FilesizeApprox
				}

				// Keep the largest size for each resolution (worst case estimate)
				if size > 0 {
					existingSize := parseSizeString(resMap[res])
					if size > existingSize {
						resMap[res] = formatBytes(float64(size))
					}
				}

			}
		}

		// Build options with available resolutions
		opts := []string{}
		for _, r := range resolutions {
			size := "Unknown"
			if s, ok := resMap[r]; ok {
				size = s
			}
			opts = append(opts, fmt.Sprintf("%s (%s)", r, size))
		}

		resSelect.Options = opts
		if len(opts) > 0 {
			// Check if current selection is still valid in new options
			currentSelection := resSelect.Selected
			selectionValid := false
			for _, opt := range opts {
				if opt == currentSelection {
					selectionValid = true
					break
				}
			}

			// Only auto-select if current selection is empty or invalid
			if !selectionValid || currentSelection == "" {
				// Select best available quality by default (prefer 1080p, then 720p, then first)
				selectedIdx := 0
				for i, opt := range opts {
					if strings.Contains(opt, "1080p") || strings.Contains(opt, "720p") {
						selectedIdx = i
						break
					}
				}
				resSelect.SetSelected(opts[selectedIdx])
			}
			resSelect.Enable()
			downloadBtn.Enable()
			statusLabel.SetText("Ready to download")
		} else {
			statusLabel.SetText("No video formats found")
		}

	})
}
>>>>>>> 5c9da659bc26ab9faf682fb549dc7e0b599a7169
