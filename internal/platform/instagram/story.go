package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/PiotrWarzachowski/go-instagram-cli/internal/video"
)

func (c *Client) GetMyStories(ctx context.Context) (*StorySummary, error) {
	if c.UserID() == 0 || c.GetSessionID() == "" {
		return nil, fmt.Errorf("not logged in")
	}

	stories, err := c.fetchUserStories(ctx, c.UserID())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stories: %w", err)
	}

	summary := &StorySummary{
		TotalStories: len(stories),
		Stories:      make([]StoryStats, len(stories)), // Pre-allocate
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10)

	for i, story := range stories {
		index := i
		s := story

		g.Go(func() error {
			stats := StoryStats{
				ID:        s.ID,
				MediaType: getMediaTypeString(s.MediaType),
				PostedAt:  time.Unix(s.TakenAt, 0),
				ExpiresAt: time.Unix(s.ExpiringAt, 0),
				ViewCount: s.TotalViewerCount,
			}

			remaining := time.Until(stats.ExpiresAt)
			if remaining > 0 {
				stats.TimeRemaining = fmt.Sprintf("%dh %dm", int(remaining.Hours()), int(remaining.Minutes())%60)
			} else {
				stats.TimeRemaining = "Expired"
			}

			viewers, totalCount, err := c.getStoryViewers(ctx, s.ID)
			if err == nil {
				stats.Viewers = viewers
				if totalCount > 0 {
					stats.ViewCount = totalCount
				}
			}

			summary.Stories[index] = stats
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, s := range summary.Stories {
		summary.TotalViews += s.ViewCount
	}

	return summary, nil
}

func (c *Client) fetchUserStories(ctx context.Context, userID int64) ([]Story, error) {
	url := fmt.Sprintf("https://www.instagram.com/api/v1/feed/user/%d/story/", userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result StoryFeedResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse story feed: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("instagram api error: %s", result.Status)
	}

	stories := make([]Story, 0, len(result.Reel.Items))
	for _, item := range result.Reel.Items {
		stories = append(stories, c.mapItemToStory(item))
	}

	return stories, nil
}

func (c *Client) mapItemToStory(item StoryItem) Story {
	viewCount := item.TotalViewerCount
	if item.TotalUniqueViewerCount > viewCount {
		viewCount = item.TotalUniqueViewerCount
	}
	if viewCount == 0 {
		viewCount = item.ViewerCount
	}

	s := Story{
		ID:               item.ID,
		MediaType:        item.MediaType,
		TakenAt:          item.TakenAt,
		ExpiringAt:       item.ExpiringAt,
		ViewerCount:      viewCount,
		TotalViewerCount: viewCount,
		VideoDuration:    item.VideoDuration,
	}

	if len(item.ImageVersions2.Candidates) > 0 {
		s.ImageURL = item.ImageVersions2.Candidates[0].URL
	}
	if len(item.VideoVersions) > 0 {
		s.VideoURL = item.VideoVersions[0].URL
	}
	if item.Caption != nil {
		s.Caption = item.Caption.Text
	}

	return s
}

func (c *Client) getStoryViewers(ctx context.Context, storyID string) ([]StoryViewer, int, error) {
	url := fmt.Sprintf("https://www.instagram.com/api/v1/media/%s/list_reel_media_viewer/", storyID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result StoryViewersResponse

	if c.Debug {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[DEBUG] Story viewers response: %s\n", string(body))
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, 0, fmt.Errorf("failed to parse viewers debug: %w", err)
		}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, 0, fmt.Errorf("failed to decode viewers: %w", err)
		}
	}

	if result.Status != "ok" {
		return nil, 0, fmt.Errorf("failed to fetch viewers: status=%s", result.Status)
	}

	return result.Users, result.TotalViewerCount, nil
}

func (c *Client) rawUploadVideo(ctx context.Context, info video.VideoInfo, pr ProgressReporter, current, total int) (string, error) {
	uploadID := strconv.FormatInt(time.Now().UnixMilli(), 10)
	waterfallID := uuid.New().String()
	uploadName := fmt.Sprintf("%s_0_%d", uploadID, rand.Int63n(9000000000)+1000000000)

	ruploadParams := map[string]string{
		"retry_context":            `{"num_step_auto_retry":0,"num_reupload":0,"num_step_manual_retry":0}`,
		"media_type":               "2",
		"upload_id":                uploadID,
		"upload_media_duration_ms": strconv.Itoa(int(info.Duration * 1000)),
		"upload_media_width":       strconv.Itoa(info.Width),
		"upload_media_height":      strconv.Itoa(info.Height),
		"for_album":                "1",
		"extract_cover_frame":      "1",
		"content_tags":             "has-overlay",
	}

	paramsJSON, _ := json.Marshal(ruploadParams)
	url := fmt.Sprintf("https://i.instagram.com/rupload_igvideo/%s", uploadName)

	// 1. Context-aware Handshake (GET)
	getReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	getReq.Header.Set("X-Instagram-Rupload-Params", string(paramsJSON))
	getReq.Header.Set("X_FB_VIDEO_WATERFALL_ID", waterfallID)
	getReq.Header.Set("Accept-Encoding", "gzip, deflate")
	c.setWebUploadHeaders(getReq)

	getResp, err := c.httpClient.Do(getReq)
	if err != nil {
		return "", fmt.Errorf("handshake network error: %w", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("handshake failed with status %d", getResp.StatusCode)
	}

	// 2. Stream video from disk instead of reading it all into RAM
	file, err := os.Open(info.Path)
	if err != nil {
		return "", fmt.Errorf("failed to open video file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	pw := &progressWriter{
		reader: file,
		total:  fileInfo.Size(),
		onProg: func(read, total int64) {
			if pr != nil {
				pr.Report(ProgressReport{
					Step:       "UPLOAD",
					Current:    int(current),
					Total:      int(total),
					BytesSent:  read,         // The 'read' from progressWriter
					TotalBytes: int64(total), // The 'total' from progressWriter
				})
			}
		},
	}

	postReq, err := http.NewRequestWithContext(ctx, "POST", url, pw)
	if err != nil {
		return "", err
	}
	postReq.ContentLength = fileInfo.Size()
	postReq.Header.Set("X-Entity-Name", uploadName)
	postReq.Header.Set("X-Entity-Length", strconv.FormatInt(fileInfo.Size(), 10))
	postReq.Header.Set("X-Entity-Type", "video/mp4")
	postReq.Header.Set("Offset", "0")
	postReq.Header.Set("Content-Type", "application/octet-stream")
	postReq.Header.Set("X-Instagram-Rupload-Params", string(paramsJSON))
	postReq.Header.Set("X_FB_VIDEO_WATERFALL_ID", waterfallID)
	c.setWebUploadHeaders(postReq) // Ensure headers are consistent

	postResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("upload network error: %w", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(postResp.Body)
		return "", fmt.Errorf("upload failed (%d): %s", postResp.StatusCode, string(body))
	}

	return uploadID, nil
}

func (c *Client) configureStory(ctx context.Context, uploadID string, info video.VideoInfo) error {
	apiURL := "https://i.instagram.com/api/v1/media/configure_to_story/?video=1"

	data := url.Values{}
	data.Set("_uid", strconv.FormatInt(c.UserID(), 10))
	data.Set("_uuid", c.UUID)
	data.Set("upload_id", uploadID)
	data.Set("source_type", "3")
	data.Set("configure_mode", "1")
	data.Set("client_timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	data.Set("camera_session_id", c.UUID)
	data.Set("creation_surface", "camera")
	data.Set("original_media_type", "video")
	data.Set("length", fmt.Sprintf("%.0f", info.Duration))

	deviceInfo, _ := json.Marshal(map[string]string{
		"manufacturer":        "Samsung",
		"model":               "SM-G973F",
		"android_version":     "29",
		"android_sdk_version": "29",
	})
	data.Set("device", string(deviceInfo))

	clips, _ := json.Marshal([]map[string]interface{}{
		{"length": info.Duration, "source_type": "3"},
	})
	data.Set("clips", string(clips))

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
			if err != nil {
				return err
			}
			c.setMobileHeaders(req)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("network error during configure: %w", err)
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}

			if strings.Contains(string(body), "transcode_not_finished") ||
				strings.Contains(string(body), "Transcode not finished yet") {
				continue
			}

			return fmt.Errorf("configure failed (Status %d): %s", resp.StatusCode, string(body))
		}
	}
}
func (c *Client) UploadStory(ctx context.Context, videoPath string, pr ProgressReporter) (*StoryPostResult, error) {
	// 1. Notify UI that video processing has started
	if pr != nil {
		pr.Report(ProgressReport{
			Type:    ProgressStory,
			Step:    "PREPARE",
			Message: "Splitting video into 15s segments...",
		})
	}

	segments, tmpDir, err := video.PrepareVideo(ctx, videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare video: %w", err)
	}

	var totalJobBytes int64
	for _, seg := range segments {
		stat, err := os.Stat(seg.Path)
		if err == nil {
			totalJobBytes += stat.Size()
		}
	}

	pr.Report(ProgressReport{
		Step:       "INIT",
		TotalBytes: totalJobBytes,
		Total:      int(totalJobBytes),
	})

	defer os.RemoveAll(tmpDir)

	res := &StoryPostResult{
		TotalParts: len(segments),
	}

	// 2. Process segments sequentially
	for i, seg := range segments {
		current := i + 1
		total := len(segments)

		// Stop if the user cancelled (Ctrl+C)
		if err := ctx.Err(); err != nil {
			res.Errors = append(res.Errors, err)
			break
		}

		// 3. Upload Step (includes Percent via progressWriter)
		uploadID, err := c.rawUploadVideo(ctx, seg, pr, current, total)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("upload failed for part %d: %w", current, err))
			continue
		}

		// 4. Configure Step
		if pr != nil {
			pr.Report(ProgressReport{
				Type:    ProgressStory,
				Step:    "CONFIG",
				Current: current,
				Total:   total,
				Message: "Configuring story on Instagram",
			})
		}

		err = c.configureStory(ctx, uploadID, seg)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("config failed for part %d: %w", current, err))
			continue
		}

		res.PartsPosted++
	}

	res.Success = res.PartsPosted == res.TotalParts
	return res, nil
}

func getMediaTypeString(mediaType int) string {
	types := map[int]string{
		1: "Photo",
		2: "Video",
		8: "Carousel", // Added for completeness
	}

	if val, ok := types[mediaType]; ok {
		return val
	}
	return "Unknown"
}
