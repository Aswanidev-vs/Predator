package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Edge cases for buildMergerArgsList that the happy-path table tests don't cover.

func TestBuildMergerArgsList_UnknownFormatFallsBackToReencode(t *testing.T) {
	// Any format we don't explicitly recognize must take the audio-re-encode path,
	// never the copy-fast path, so we don't try to copy an incompatible codec into mp4.
	got := buildMergerArgsList("flac")
	assert.Equal(t, "-c:v copy -c:a aac -b:a 192k", got[0])
	assert.Equal(t, fullReencodeFallback, got[len(got)-1])
}

func TestBuildMergerArgsList_CaseSensitivity(t *testing.T) {
	// AudioFormat comes lowercase from the UI, but the match must be
	// case-insensitive so an uppercase M4A/AAC still takes the safe copy-fast
	// path instead of being forced through an unnecessary audio re-encode.
	upper := buildMergerArgsList("M4A")
	lower := buildMergerArgsList("m4a")
	assert.Equal(t, lower, upper, "M4A and m4a must produce the same ladder")
	assert.Equal(t, "-c:v copy -c:a copy", upper[0])
}

func TestBuildMergerArgsList_NeverEmpty(t *testing.T) {
	// The ladder must always contain at least the guaranteed libx264 re-encode fallback,
	// otherwise the merge loop could complete with no output.
	for _, fmt := range []string{"m4a", "aac", "mp3", "opus", "wav", "", "flac", "M4A"} {
		got := buildMergerArgsList(fmt)
		assert.NotEmpty(t, got, "ladder must never be empty for %q", fmt)
		assert.Equal(t, fullReencodeFallback, got[len(got)-1], "must always end with re-encode for %q", fmt)
	}
}
