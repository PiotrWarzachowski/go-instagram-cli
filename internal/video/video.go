package video

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

type VideoInfo struct {
	Path      string
	Width     int
	Height    int
	Duration  float64
	Thumbnail string
}

func probeVideo(path string) (int, int, float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,duration",
		"-of", "csv=p=0", path)

	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid ffprobe output")
	}

	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	d, _ := strconv.ParseFloat(parts[2], 64)

	return w, h, d, nil
}

func getTotalDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get total duration: %w", err)
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

func PrepareVideo(ctx context.Context, inputPath string) ([]VideoInfo, string, error) {
	totalDuration, err := getTotalDuration(inputPath)
	if err != nil {
		return nil, "", err
	}

	tmpDir, err := os.MkdirTemp("", "story_upload")
	if err != nil {
		return nil, "", err
	}

	const segmentLen = 58.0
	numSegments := int(math.Ceil(totalDuration / segmentLen))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	var mu sync.Mutex
	processed := make([]VideoInfo, 0, numSegments)

	for i := 0; i < numSegments; i++ {
		index := i
		start := float64(i) * segmentLen

		g.Go(func() error {
			outputPath := filepath.Join(tmpDir, fmt.Sprintf("segment_%03d.mp4", index))
			thumbPath := outputPath + ".jpg"

			cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
				"-ss", fmt.Sprintf("%f", start),
				"-t", fmt.Sprintf("%f", segmentLen),
				"-i", inputPath,
				"-c:v", "libx264",
				"-preset", "veryfast",
				"-crf", "22",
				"-c:a", "aac",
				"-b:a", "128k",
				"-avoid_negative_ts", "make_zero",
				outputPath)

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("segment %d failed: %w", index, err)
			}

			w, h, d, err := probeVideo(outputPath)
			if err != nil {
				return err
			}

			_ = exec.CommandContext(ctx, "ffmpeg", "-i", outputPath, "-ss", "0.5", "-vframes", "1", thumbPath).Run()

			mu.Lock()
			processed = append(processed, VideoInfo{
				Path:      outputPath,
				Width:     w,
				Height:    h,
				Duration:  d,
				Thumbnail: thumbPath,
			})
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", err
	}

	return processed, tmpDir, nil
}
