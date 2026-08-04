package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// fullReencodeFallback is the guaranteed-last step of the merge ladder: it always
// re-encodes to H.264/AAC so a valid MP4 is produced regardless of the source codecs.
const fullReencodeFallback = "-c:v libx264 -crf 18 -c:a aac -b:a 192k"

func TestBuildMergerArgsList(t *testing.T) {
	tests := []struct {
		name        string
		audioFormat string
		wantFirst   string
		wantLen     int
	}{
		{"m4a keeps copy-fast path", "m4a", "-c:v copy -c:a copy", 3},
		{"aac keeps copy-fast path", "aac", "-c:v copy -c:a copy", 3},
		{"mp3 re-encodes audio first", "mp3", "-c:v copy -c:a aac -b:a 192k", 2},
		{"opus re-encodes audio first", "opus", "-c:v copy -c:a aac -b:a 192k", 2},
		{"wav re-encodes audio first", "wav", "-c:v copy -c:a aac -b:a 192k", 2},
		{"empty defaults to re-encode audio", "", "-c:v copy -c:a aac -b:a 192k", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMergerArgsList(tt.audioFormat)

			require.NotEmpty(t, got, "ladder must never be empty")
			assert.Equal(t, tt.wantFirst, got[0], "unexpected first step")
			assert.Len(t, got, tt.wantLen)
			// Core invariant of the fix: the final step always re-encodes to H.264/AAC.
			assert.Equal(t, fullReencodeFallback, got[len(got)-1],
				"last fallback must always re-encode so a valid MP4 is produced")
			// The full re-encode must appear exactly once (no duplicate trailing step).
			assert.Equal(t, 1, countOccurrences(got, fullReencodeFallback),
				"full re-encode fallback should appear exactly once")
		})
	}
}

func TestBuildMergerArgsList_M4AFallbackProgression(t *testing.T) {
	got := buildMergerArgsList("m4a")

	require.Len(t, got, 3)
	assert.Equal(t, "-c:v copy -c:a copy", got[0])
	assert.Equal(t, "-c:v copy -c:a aac -b:a 192k", got[1])
	assert.Equal(t, fullReencodeFallback, got[2])
	// Each step progressively does more work: only the last re-encodes the video.
	for i, step := range got[:len(got)-1] {
		assert.NotContains(t, step, "libx264", "step %d should not re-encode video", i)
	}
}

func TestBuildAudioFormatString(t *testing.T) {
	tests := []struct {
		format   string
		contains []string
		excludes []string
	}{
		{"m4a", []string{"ext=m4a", "mp4a"}, nil},
		{"opus", []string{"opus", "webm"}, nil},
		{"mp3", []string{"bestaudio/best"}, []string{"ext=m4a"}},
		{"wav", []string{"bestaudio/best"}, nil},
		{"", []string{"bestaudio/best"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := buildAudioFormatString(tt.format)
			require.NotEmpty(t, got)
			for _, c := range tt.contains {
				assert.Contains(t, got, c)
			}
			for _, e := range tt.excludes {
				assert.NotContains(t, got, e)
			}
		})
	}
}

func TestBuildVideoFormatString(t *testing.T) {
	// "best" and a numeric resolution must both prefer H.264 with m4a audio and never
	// request opus/webm, which would break MP4 merging.
	for _, res := range []string{"best", "1080", "720"} {
		t.Run(res, func(t *testing.T) {
			got := buildVideoFormatString(res, true)
			assert.Contains(t, got, "vcodec^=avc1", "must prefer H.264")
			assert.Contains(t, got, "ext=m4a", "must prefer m4a audio for MP4")
			assert.NotContains(t, got, "opus", "must not select opus for MP4 merge")
			assert.NotContains(t, got, "webm", "must not select webm video for MP4 merge")
			// Codec preference order must be H.264 (avc1) before VP9 before AV1.
			avc1 := strings.Index(got, "vcodec^=avc1")
			vp9 := strings.Index(got, "vcodec^=vp9")
			av01 := strings.Index(got, "vcodec^=av01")
			require.True(t, avc1 >= 0 && vp9 >= 0 && av01 >= 0)
			assert.Less(t, avc1, vp9, "H.264 should be preferred over VP9")
			assert.Less(t, vp9, av01, "VP9 should be preferred over AV1")
			// m4a audio must be tried before the looser mp4a match for the leading H.264 entry.
			assert.Less(t, strings.Index(got, "ext=m4a"), strings.Index(got, "acodec^=mp4a"),
				"m4a should be tried before the looser mp4a match")
		})
	}
}

func TestBuildWebmFormatString(t *testing.T) {
	got := buildWebmFormatString("1080")
	assert.Contains(t, got, "av01", "webm should prefer AV1")
	assert.Contains(t, got, "vp9", "webm should support VP9")
	assert.Contains(t, got, "opus", "webm should use opus audio")
	assert.NotContains(t, got, "ext=m4a", "webm should not request m4a audio")
}

func countOccurrences(haystack []string, needle string) int {
	n := 0
	for _, s := range haystack {
		if s == needle {
			n++
		}
	}
	return n
}
