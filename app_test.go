package main

import (
	"os"
	"path/filepath"
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

func TestIsCandidateFile(t *testing.T) {
	finals := []string{
		"My Video [dQw4w9WgXcQ] (1080p).mp4",
		"My Video [dQw4w9WgXcQ].m4a",
		"Some Song [abc123].mp3",
	}
	for _, name := range finals {
		assert.True(t, isCandidateFile(name), "%q is a final output", name)
	}

	intermediates := []string{
		"My Video [dQw4w9WgXcQ] (1080p).f137.mp4.part", // part file
		"My Video [dQw4w9WgXcQ].ytdl",                  // incomplete marker
		"My Video [dQw4w9WgXcQ].temp.mp4",              // merger scratch
		"My Video [dQw4w9WgXcQ] (1080p).f137.mp4",      // orphan stream fragment
		"My Video [dQw4w9WgXcQ] (1080p).f140.m4a",      // orphan audio fragment
	}
	for _, name := range intermediates {
		assert.False(t, isCandidateFile(name), "%q is an intermediate, not a final", name)
	}
}

func TestFindCompletedFile_IgnoresFragmentsAndPrefersType(t *testing.T) {
	dir := t.TempDir()

	// Orphan fragment from a failed merge attempt (must be ignored).
	fragment := filepath.Join(dir, "Clip [dQw4w9WgXcQ] (720p).f140.m4a")
	// Loose audio grab of the same video (must lose to the video output).
	looseAudio := filepath.Join(dir, "Clip [dQw4w9WgXcQ].m4a")
	// The real merged output.
	merged := filepath.Join(dir, "Clip [dQw4w9WgXcQ] (720p).mp4")
	for _, p := range []string{fragment, looseAudio, merged} {
		require.NoError(t, os.WriteFile(p, []byte("x"), 0644))
	}

	// Video task: must pick the merged mp4, never the fragment or loose audio.
	path, size := findCompletedFile(dir, "dQw4w9WgXcQ", "Video")
	assert.Equal(t, merged, path, "video task must resolve to the merged mp4")
	assert.Greater(t, size, int64(0))

	// Audio task on the same folder: must pick the loose m4a, not the fragment
	// and not the mp4.
	path, _ = findCompletedFile(dir, "dQw4w9WgXcQ", "Audio")
	assert.Equal(t, looseAudio, path, "audio task must resolve to the .m4a final")
}

func TestFindCompletedFile_NoneExists(t *testing.T) {
	dir := t.TempDir()
	path, size := findCompletedFile(dir, "nope123", "Video")
	assert.Empty(t, path)
	assert.Equal(t, int64(0), size)
}

func TestExtractErrorDetail_StripsLeadingWarnings(t *testing.T) {
	// Real-world shape of go-ytdlp's error string: routine warnings first,
	// the actual ERROR line last. The user must see the cause, not the warning.
	errStr := "exit code 1: exit status 1\n" +
		"WARNING: Your yt-dlp version (2026.03.17) is older than 90 days!\n" +
		"         It is strongly recommended to always use the latest version.\n" +
		"[youtube] dQw4w9WgXcQ: Downloading webpage\n" +
		"ERROR: [youtube] dQw4w9WgXcQ: Video unavailable\n"

	assert.Equal(t, "ERROR: [youtube] dQw4w9WgXcQ: Video unavailable", extractErrorDetail(errStr))
}

func TestExtractErrorDetail_PrefersLastErrorLine(t *testing.T) {
	errStr := "ERROR: first problem\n" +
		"WARNING: unrelated noise\n" +
		"ERROR: final merge failure\n"

	assert.Equal(t, "ERROR: final merge failure", extractErrorDetail(errStr))
}

func TestExtractErrorDetail_PostprocessingWinsWithoutErrorLine(t *testing.T) {
	errStr := "WARNING: stale version\n" +
		"Postprocessing: ffmpeg exited with code 1\n"

	assert.Equal(t, "Postprocessing: ffmpeg exited with code 1", extractErrorDetail(errStr))
}

func TestExtractErrorDetail_FallsBackToLastNonEmptyLine(t *testing.T) {
	assert.Equal(t, "some raw failure", extractErrorDetail("noise\nsome raw failure\n"))
	assert.Equal(t, "single line", extractErrorDetail("single line"))
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
