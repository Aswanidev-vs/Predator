package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/lrstanley/go-ytdlp"
)

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

	downloadType := widget.NewRadioGroup([]string{"Video", "Audio"}, nil)
	downloadType.SetSelected("Video")

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

	progressBar := widget.NewProgressBar()
	statusLabel := widget.NewLabel("Idle")
	speedLabel := widget.NewLabel("")

	downloadBtn := widget.NewButton("Download", nil)
	downloadBtn.Disable()

	cancelBtn := widget.NewButton("Cancel", nil)
	cancelBtn.Disable()

	titleLabel := widget.NewLabel("")
	titleLabel.Wrapping = fyne.TextWrapWord
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
			return
		}

		fetchTimer = time.AfterFunc(600*time.Millisecond, func() {
			if atomic.LoadInt32(&fetching) == 1 {
				return
			}
			atomic.StoreInt32(&fetching, 1)

			fyne.Do(func() {
				statusLabel.SetText("Fetching video info...")
				resSelect.Disable()
				downloadBtn.Disable()
			})

			go fetchVideoInfo(text, resolutions, resSelect, statusLabel, titleLabel, downloadBtn, &fetching)
		})
	}

	/* -------------------- Download -------------------- */

	var cancelFunc context.CancelFunc
	var downloading int32

	downloadBtn.OnTapped = func() {
		if atomic.LoadInt32(&downloading) == 1 {
			return
		}

		url := strings.TrimSpace(urlEntry.Text)
		if url == "" {
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel
		atomic.StoreInt32(&downloading, 1)

		progressBar.SetValue(0)
		statusLabel.SetText("Starting download...")
		cancelBtn.Enable()
		downloadBtn.Disable()

		go func() {
			defer atomic.StoreInt32(&downloading, 0)

			var lastDownloaded int
			var lastTime time.Time = time.Now()
			var smoothedSpeed float64

			updateProgress := func(p ytdlp.ProgressUpdate) {
				if p.Status != "downloading" {
					return
				}

				now := time.Now()
				elapsed := now.Sub(lastTime).Seconds()
				var speed float64
				if elapsed > 0 {
					speed = float64(p.DownloadedBytes-lastDownloaded) / elapsed
				}

				// Apply exponential moving average for smoother speed display
				const alpha = 0.2 // Smoothing factor: 20% weight to new speed
				if smoothedSpeed == 0 {
					smoothedSpeed = speed
				} else {
					smoothedSpeed = alpha*speed + (1-alpha)*smoothedSpeed
				}
				fyne.Do(func() {
					progressBar.SetValue(p.Percent() / 100)

					var downloadFinished bool

					// ---- Detect download completion ----
					if p.Percent() >= 100 && p.Status != "downloading" {
						downloadFinished = true
					}

					// ---- Processing / ffmpeg phase ----
					if downloadFinished {
						statusLabel.SetText("Processing…")
						speedLabel.SetText("")
						return
					}

					// ---- Normal downloading ----
					statusLabel.SetText(fmt.Sprintf("Downloading… %.1f%%", p.Percent()))

					displaySpeed := smoothedSpeed
					if displaySpeed < 0 {
						displaySpeed = 0
					}

					speedLabel.SetText(fmt.Sprintf(
						"Speed: %s | ETA: %s",
						formatSpeed(displaySpeed),
						formatETA(p.ETA()),
					))
				})

				lastDownloaded = p.DownloadedBytes
				lastTime = now
			}

			var err error

			if downloadType.Selected == "Video" {
				selected := strings.Split(resSelect.Selected, " ")[0]
				res := strings.TrimSuffix(selected, "p")

				var format string
				if selected != "best" {
					format = fmt.Sprintf(
						"bestvideo[ext=mp4][height<=%s]+bestaudio[ext=m4a]/mp4/"+
							"bestvideo[height<=%s]+bestaudio/best",
						res, res,
					)
				} else {
					format = "bestvideo[ext=mp4]+bestaudio[ext=m4a]/mp4/bestvideo+bestaudio/best"
				}

				_, err = ytdlp.New().
					Format(format).
					MergeOutputFormat("mp4").
					Output(outputDir+"/%(title)s.%(ext)s").
					ProgressFunc(200*time.Millisecond, updateProgress).
					Run(ctx, url)

			} else {
				_, err = ytdlp.New().
					ExtractAudio().
					AudioFormat(audioSelect.Selected).
					Output(outputDir+"/%(title)s.%(ext)s").
					ProgressFunc(200*time.Millisecond, updateProgress).
					Run(ctx, url)
			}

			fyne.Do(func() {
				cancelBtn.Disable()
				downloadBtn.Enable()
				speedLabel.SetText("")

				if err != nil {
					if ctx.Err() == context.Canceled {
						statusLabel.SetText("Download canceled")
					} else {
						log.Println(err)
						statusLabel.SetText("Download failed")
					}
					progressBar.SetValue(0)
				} else {
					progressBar.SetValue(1)
					statusLabel.SetText("Download completed")
				}
			})
		}()
	}

	cancelBtn.OnTapped = func() {
		if cancelFunc != nil {
			cancelFunc()
		}
	}

	/* -------------------- Layout -------------------- */

	content := container.NewVBox(
		widget.NewLabel("YouTube URL"),
		urlEntry,
		titleLabel,
		downloadType,
		resSelect,
		audioSelect,
		outputDirLabel,
		changeDirBtn,
		container.NewHBox(downloadBtn, cancelBtn),
		statusLabel,
		speedLabel,
		progressBar,
	)

	w.SetContent(container.NewScroll(content))
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := ytdlp.New().DumpJSON().Run(ctx, url)

	fyne.Do(func() {
		defer atomic.StoreInt32(fetching, 0)

		if err != nil {
			statusLabel.SetText("Failed to fetch info")
			return
		}

		var info struct {
			Title   string `json:"title"`
			Formats []struct {
				Height         *int   `json:"height"`
				Filesize       *int64 `json:"filesize"`
				FilesizeApprox *int64 `json:"filesize_approx"`
				Vcodec         string `json:"vcodec"`
			} `json:"formats"`
		}

		if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
			statusLabel.SetText("Failed to parse info")
			return
		}
		titleLabel.SetText("Title : " + info.Title)
		resMap := make(map[string]string)
		for _, f := range info.Formats {
			if f.Vcodec != "none" && f.Height != nil {
				res := fmt.Sprintf("%dp", *f.Height)
				if f.Filesize != nil {
					resMap[res] = formatBytes(float64(*f.Filesize))
				} else if f.FilesizeApprox != nil {
					resMap[res] = "~" + formatBytes(float64(*f.FilesizeApprox))
				}
			}
		}

		opts := []string{}
		for _, r := range resolutions {
			size := "Unknown"
			if s, ok := resMap[r]; ok {
				size = s
			}
			opts = append(opts, fmt.Sprintf("%s (%s)", r, size))
		}

		resSelect.Options = opts
		resSelect.SetSelected(opts[0])
		resSelect.Enable()
		downloadBtn.Enable()
		statusLabel.SetText("Ready to download")
	})
}
