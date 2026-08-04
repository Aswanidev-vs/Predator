package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the REAL ffmpeg merge ladder end-to-end (no network, no yt-dlp).
// They generate tiny synthetic media with ffmpeg and run the exact codec arg strings
// that buildMergerArgsList produces, proving the ladder yields a valid MP4 for any
// source encoding. They are skipped if ffmpeg is not on PATH.

func ffmpegBin(t *testing.T) string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH; skipping integration test")
	}
	return p
}

// genMedia runs ffmpeg to produce one synthetic file from a lavfi source.
func genMedia(t *testing.T, ffmpeg, name, filter, codecArgs string) string {
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command(ffmpeg, "-y", "-f", "lavfi", "-i", filter, "-t", "1")
	cmd.Args = append(cmd.Args, strings.Fields(codecArgs)...)
	cmd.Args = append(cmd.Args, out)
	cmdOut, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to generate %s:\n%s", name, cmdOut)
	return out
}

// tryMergeStep runs a single ladder step (merging video+audio into out).
func tryMergeStep(ffmpeg, video, audio, step, out string) error {
	cmd := exec.Command(ffmpeg, "-y", "-i", video, "-i", audio)
	cmd.Args = append(cmd.Args, strings.Fields(step)...)
	cmd.Args = append(cmd.Args, out)
	return cmd.Run()
}

// runMergeLadder mirrors app.go's retry loop: try each step until one succeeds.
func runMergeLadder(ffmpeg, video, audio, out, audioFormat string) error {
	for _, step := range buildMergerArgsList(audioFormat) {
		if err := tryMergeStep(ffmpeg, video, audio, step, out); err == nil {
			return nil
		}
		_ = os.Remove(out)
	}
	return errors.New("all merge steps failed")
}

// assertValidMP4 checks the file is non-empty and (when ffprobe is present) contains
// both a video and an audio stream.
func assertValidMP4(t *testing.T, path string) {
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "output must be non-empty")

	if probe, perr := exec.LookPath("ffprobe"); perr == nil {
		out, perr := exec.Command(probe, "-v", "error",
			"-show_entries", "stream=codec_type", "-of", "csv=p=0", path).Output()
		require.NoError(t, perr, "ffprobe failed")
		s := string(out)
		assert.Contains(t, s, "video", "expected a video stream in %s", path)
		assert.Contains(t, s, "audio", "expected an audio stream in %s", path)
	}
}

// Regression test: H.264 video + AAC/m4a audio should merge with the fast copy path
// (step 1), proving the common case needs no re-encode.
func TestMergeLadder_CopyFastPath(t *testing.T) {
	ffmpeg := ffmpegBin(t)
	video := genMedia(t, ffmpeg, "vid_h264.mp4",
		"testsrc=duration=1:size=128x128:rate=10", "-pix_fmt yuv420p -c:v libx264")
	audio := genMedia(t, ffmpeg, "aud_aac.m4a",
		"sine=frequency=440:duration=1", "-c:a aac")

	out := filepath.Join(t.TempDir(), "out.mp4")

	// Document that the fast copy path alone succeeds for compatible inputs.
	require.NoError(t, tryMergeStep(ffmpeg, video, audio, buildMergerArgsList("m4a")[0], out),
		"step 1 (copy) should succeed for H.264+AAC")

	// The full ladder must also succeed and produce a valid MP4.
	require.NoError(t, runMergeLadder(ffmpeg, video, audio, out, "m4a"))
	assertValidMP4(t, out)
}

// Regression test for the m4a merging bug: a video codec that MP4 cannot mux by copy
// (here VP8) paired with m4a audio must fail the copy steps and succeed via the
// libx264 fallback. This is exactly the scenario that previously produced a hard merge
// failure with no recovery. (VP9/AV1/H.264 copy fine into MP4; VP8/Theora/FFV1 do not.)
func TestMergeLadder_IncompatibleVideoRequiresReencode(t *testing.T) {
	ffmpeg := ffmpegBin(t)
	video := genMedia(t, ffmpeg, "vid_vp8.webm",
		"testsrc=duration=1:size=128x128:rate=10", "-pix_fmt yuv420p -c:v vp8")
	audio := genMedia(t, ffmpeg, "aud_aac.m4a",
		"sine=frequency=440:duration=1", "-c:a aac")

	out := filepath.Join(t.TempDir(), "out.mp4")
	ladder := buildMergerArgsList("m4a")

	// The copy paths must fail for a VP8 source (this is the bug condition).
	assert.Error(t, tryMergeStep(ffmpeg, video, audio, ladder[0], out),
		"copy-fast path should fail for VP8 video")
	assert.Error(t, tryMergeStep(ffmpeg, video, audio, ladder[1], out),
		"audio-only re-encode should still fail because video is VP8")

	// The libx264 fallback must succeed and yield a valid MP4.
	require.NoError(t, runMergeLadder(ffmpeg, video, audio, out, "m4a"))
	assertValidMP4(t, out)
}

// Opus (webm) audio must be re-encoded to AAC; the video can still be copied.
func TestMergeLadder_OpusAudioReencoded(t *testing.T) {
	ffmpeg := ffmpegBin(t)
	video := genMedia(t, ffmpeg, "vid_h264.mp4",
		"testsrc=duration=1:size=128x128:rate=10", "-pix_fmt yuv420p -c:v libx264")
	audio := genMedia(t, ffmpeg, "aud_opus.webm",
		"sine=frequency=440:duration=1", "-c:a libopus")

	out := filepath.Join(t.TempDir(), "out.mp4")
	require.NoError(t, runMergeLadder(ffmpeg, video, audio, out, "opus"))
	assertValidMP4(t, out)
}
