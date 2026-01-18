package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// StoryUploadResponse represents the response from story upload
type StoryUploadResponse struct {
	Media struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Pk   int64  `json:"pk"`
	} `json:"media"`
	Status string `json:"status"`
}

// VideoInfo contains video metadata
type VideoInfo struct {
	Duration float64
	Width    int
	Height   int
	Codec    string
}

// StoryPostResult represents the result of posting a story
type StoryPostResult struct {
	Success     bool
	MediaID     string
	PartsPosted int
	TotalParts  int
	Error       error
}

// MaxStoryDuration is the maximum duration for a single story video (60 seconds)
const MaxStoryDuration = 60

// PostVideoStory posts a video as a story, splitting if needed
func (c *Client) PostVideoStory(videoPath string) (*StoryPostResult, error) {
	// Check if file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", videoPath)
	}

	// Get video info
	videoInfo, err := c.getVideoInfo(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Video info: duration=%.2fs, size=%dx%d\n",
			videoInfo.Duration, videoInfo.Width, videoInfo.Height)
	}

	// If video is under 60 seconds, upload directly
	if videoInfo.Duration <= MaxStoryDuration {
		resp, err := c.uploadVideoStory(videoPath)
		if err != nil {
			return &StoryPostResult{Success: false, Error: err}, err
		}
		return &StoryPostResult{
			Success:     true,
			MediaID:     resp.Media.ID,
			PartsPosted: 1,
			TotalParts:  1,
		}, nil
	}

	// Split video into parts
	fmt.Printf("📹 Video is %.0f seconds, splitting into %d parts...\n",
		videoInfo.Duration, int(videoInfo.Duration/MaxStoryDuration)+1)

	parts, err := c.splitVideo(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to split video: %w", err)
	}
	defer c.cleanupTempFiles(parts)

	// Upload each part
	result := &StoryPostResult{
		TotalParts: len(parts),
	}

	for i, partPath := range parts {
		fmt.Printf("📤 Uploading part %d/%d...\n", i+1, len(parts))

		resp, err := c.uploadVideoStory(partPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to upload part %d: %w", i+1, err)
			return result, result.Error
		}

		result.PartsPosted++
		if i == 0 {
			result.MediaID = resp.Media.ID
		}

		// Small delay between uploads to avoid rate limiting
		if i < len(parts)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	result.Success = true
	return result, nil
}

// getVideoInfo gets video metadata using ffprobe
func (c *Client) getVideoInfo(videoPath string) (*VideoInfo, error) {
	// Check if ffprobe is available
	_, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found. Please install FFmpeg: https://ffmpeg.org/download.html")
	}

	// Get duration
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %w", err)
	}

	// Get dimensions
	cmd = exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0:s=x",
		videoPath,
	)
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe dimensions failed: %w", err)
	}

	dimensions := strings.Split(strings.TrimSpace(string(output)), "x")
	width, height := 0, 0
	if len(dimensions) == 2 {
		width, _ = strconv.Atoi(dimensions[0])
		height, _ = strconv.Atoi(dimensions[1])
	}

	return &VideoInfo{
		Duration: duration,
		Width:    width,
		Height:   height,
	}, nil
}

// splitVideo splits a video into ~60 second parts
func (c *Client) splitVideo(videoPath string) ([]string, error) {
	// Check if ffmpeg is available
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found. Please install FFmpeg: https://ffmpeg.org/download.html")
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "instagram-story-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Get base name without extension
	baseName := filepath.Base(videoPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]

	// Output pattern
	outputPattern := filepath.Join(tempDir, nameWithoutExt+"_part_%03d.mp4")

	// Split using ffmpeg segment
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "fast",
		"-f", "segment",
		"-segment_time", "59", // Slightly under 60 to be safe
		"-reset_timestamps", "1",
		"-map", "0",
		"-y",
		outputPattern,
	)

	if c.Debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg split failed: %w", err)
	}

	// Find all created parts
	pattern := filepath.Join(tempDir, nameWithoutExt+"_part_*.mp4")
	parts, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find parts: %w", err)
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no video parts created")
	}

	return parts, nil
}

// cleanupTempFiles removes temporary video parts
func (c *Client) cleanupTempFiles(files []string) {
	for _, f := range files {
		os.Remove(f)
	}
	// Also try to remove the temp directory
	if len(files) > 0 {
		os.Remove(filepath.Dir(files[0]))
	}
}

// UploadResponse represents the response from video upload
type UploadResponse struct {
	MediaID int64  `json:"media_id"`
	Status  string `json:"status"`
}

// uploadVideoStory uploads a single video as a story
func (c *Client) uploadVideoStory(videoPath string) (*StoryUploadResponse, error) {
	// Step 1: Upload the video file
	uploadID := strconv.FormatInt(time.Now().UnixMilli(), 10)
	uploadResp, err := c.uploadVideoToInstagram(videoPath, uploadID)
	if err != nil {
		return nil, fmt.Errorf("video upload failed: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Upload response: %s\n", string(uploadResp))
	}

	// Parse upload response to get media_id
	var uploadResult UploadResponse
	if err := json.Unmarshal(uploadResp, &uploadResult); err != nil {
		return nil, fmt.Errorf("failed to parse upload response: %w", err)
	}

	if uploadResult.Status != "ok" {
		return nil, fmt.Errorf("upload failed: status=%s", uploadResult.Status)
	}

	// Step 2: Configure as story using the upload_id (not media_id)
	return c.configureVideoStory(uploadID, videoPath)
}

// uploadVideoToInstagram uploads the video file to Instagram's servers using web API
func (c *Client) uploadVideoToInstagram(videoPath string, uploadID string) ([]byte, error) {
	// Read video file
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read video: %w", err)
	}

	// Get video info for proper params
	videoInfo, _ := c.getVideoInfo(videoPath)
	durationMs := int(videoInfo.Duration * 1000)
	if durationMs == 0 {
		durationMs = 60000
	}

	// Create the upload URL - use web API endpoint
	entityName := fmt.Sprintf("%s_0_%d", uploadID, time.Now().Unix())
	uploadURL := fmt.Sprintf("https://www.instagram.com/rupload_igvideo/%s", entityName)

	// Create request with video data
	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(videoData))
	if err != nil {
		return nil, err
	}

	// Build rupload params for web - specify this is for stories
	ruploadParams := map[string]interface{}{
		"upload_id":                uploadID,
		"media_type":               "2", // 2 = video
		"xsharing_user_ids":        "[]",
		"upload_media_height":      strconv.Itoa(videoInfo.Height),
		"upload_media_width":       strconv.Itoa(videoInfo.Width),
		"upload_media_duration_ms": strconv.Itoa(durationMs),
		"for_direct_story":         "1", // Mark as story upload
		"for_album":                "0",
		"direct_v2":                "0",
		"is_unified_video":         "1",
		"is_sidecar":               "0",
	}
	ruploadJSON, _ := json.Marshal(ruploadParams)

	// Set web-style headers
	req.Header.Set("X-Entity-Type", "video/mp4")
	req.Header.Set("Offset", "0")
	req.Header.Set("X-Instagram-Rupload-Params", string(ruploadJSON))
	req.Header.Set("X-Entity-Name", entityName)
	req.Header.Set("X-Entity-Length", strconv.Itoa(len(videoData)))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(len(videoData)))

	// Use web headers instead of mobile
	c.setWebUploadHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Upload status: %d, body: %s\n", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// setWebUploadHeaders sets headers for web API upload requests
func (c *Client) setWebUploadHeaders(req *http.Request) {

	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("X-CSRFToken", c.CSRFToken())
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-Web-Device-Id", c.UUID)
	req.Header.Set("X-ASBD-ID", "359341")
	req.Header.Set("X-IG-WWW-Claim", c.IgWwwClaim)
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Referer", "https://www.instagram.com/create/story/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// Add all cookies
	var cookieStrings []string
	for name, value := range c.Cookies {
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", name, value))
	}
	if len(cookieStrings) > 0 {
		req.Header.Set("Cookie", strings.Join(cookieStrings, "; "))
	}
}

// configureVideoStory configures the uploaded video as a story using web API
func (c *Client) configureVideoStory(uploadID string, videoPath string) (*StoryUploadResponse, error) {
	// Get video info for configuration
	videoInfo, err := c.getVideoInfo(videoPath)
	if err != nil {
		return nil, err
	}

	durationSec := videoInfo.Duration
	if durationSec == 0 {
		durationSec = 60
	}

	// Small delay to allow video processing
	time.Sleep(2 * time.Second)

	width := videoInfo.Width
	height := videoInfo.Height
	if width == 0 {
		width = 1080
	}
	if height == 0 {
		height = 1920
	}

	// // Build configure data - minimal for story video
	// configData := map[string]interface{}{
	// 	"upload_id":      uploadID,
	// 	"source_type":    "3",
	// 	"configure_mode": "1",
	// 	"video_result":   "",
	// 	"caption":        "",
	// 	"audio_muted":    false,
	// 	"device": map[string]interface{}{
	// 		"manufacturer":    "Apple",
	// 		"model":           "iPhone 12",
	// 		"android_version": 24,
	// 		"android_release": "7.0",
	// 	},
	// 	"extra": map[string]interface{}{
	// 		"source_width":  videoInfo.Width,
	// 		"source_height": videoInfo.Height,
	// 	},
	// }

	// jsonData, err := json.Marshal(configData)
	// if err != nil {
	// 	return nil, err
	// }

	// if c.Debug {
	// 	fmt.Printf("[DEBUG] Configure request: %s\n", string(jsonData))
	// }
	data := url.Values{}
	data.Set("caption", "")
	data.Set("configure_mode", "1")
	data.Set("share_to_facebook", "")
	// Note: These FB IDs are session-specific; leaving them blank or
	// using your captured ones is usually fine.
	data.Set("share_to_fb_destination_id", "")
	data.Set("share_to_fb_destination_type", "USER")
	data.Set("upload_id", uploadID)
	data.Set("jazoest", "22856") // Standard web token
	// Create request to web API endpoint for video stories
	configURL := "https://www.instagram.com/api/v1/media/configure_to_story/"

	req, err := http.NewRequest("POST", configURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	c.setWebUploadHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Configure response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Configure response: %s\n", string(body))
	}

	// If first attempt fails, try alternative endpoint
	if resp.StatusCode != http.StatusOK {
		// Try the video-specific configure endpoint
		return c.configureVideoStoryAlt(uploadID, videoPath, videoInfo)
	}

	var storyResp StoryUploadResponse
	if err := json.Unmarshal(body, &storyResp); err != nil {
		return nil, fmt.Errorf("failed to parse configure response: %w", err)
	}

	if storyResp.Status != "ok" {
		return nil, fmt.Errorf("configure failed: status=%s", storyResp.Status)
	}

	return &storyResp, nil
}

// configureVideoStoryAlt tries an alternative configure approach
func (c *Client) configureVideoStoryAlt(uploadID string, videoPath string, videoInfo *VideoInfo) (*StoryUploadResponse, error) {
	durationMs := int(videoInfo.Duration * 1000)
	if durationMs == 0 {
		durationMs = 60000
	}

	width := videoInfo.Width
	height := videoInfo.Height
	if width == 0 {
		width = 1080
	}
	if height == 0 {
		height = 1920
	}

	// Try form-encoded request like mobile API
	configData := map[string]interface{}{
		"upload_id":          uploadID,
		"source_type":        "3",
		"caption":            "",
		"_uid":               strconv.FormatInt(c.UserID(), 10),
		"_uuid":              c.UUID,
		"device_id":          c.AndroidDeviceID,
		"length":             videoInfo.Duration,
		"clips":              fmt.Sprintf(`[{"length":%f,"source_type":"3"}]`, videoInfo.Duration),
		"poster_frame_index": 0,
		"audio_muted":        false,
	}

	jsonData, err := json.Marshal(configData)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Alt configure request: %s\n", string(jsonData))
	}

	configURL := "https://www.instagram.com/api/v1/media/configure_to_story/"

	req, err := http.NewRequest("POST", configURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	c.setWebUploadHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Alt configure response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Alt configure response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("configure failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var storyResp StoryUploadResponse
	if err := json.Unmarshal(body, &storyResp); err != nil {
		return nil, fmt.Errorf("failed to parse configure response: %w", err)
	}

	if storyResp.Status != "ok" {
		return nil, fmt.Errorf("configure failed: status=%s", storyResp.Status)
	}

	return &storyResp, nil
}

// PostPhotoStory posts a photo as a story
func (c *Client) PostPhotoStory(photoPath string) (*StoryUploadResponse, error) {
	// Check if file exists
	if _, err := os.Stat(photoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("photo file not found: %s", photoPath)
	}

	// Upload the photo
	uploadID := fmt.Sprintf("%d", time.Now().UnixNano()/1000000)

	uploadResp, err := c.uploadPhotoToInstagram(photoPath, uploadID)
	if err != nil {
		return nil, fmt.Errorf("photo upload failed: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Photo upload response: %s\n", string(uploadResp))
	}

	// Configure as story
	return c.configurePhotoStory(uploadID)
}

// uploadPhotoToInstagram uploads a photo to Instagram using web API
func (c *Client) uploadPhotoToInstagram(photoPath string, uploadID string) ([]byte, error) {
	// Read photo file
	photoData, err := os.ReadFile(photoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read photo: %w", err)
	}

	// Determine content type
	contentType := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(photoPath))
	if ext == ".png" {
		contentType = "image/png"
	}

	// Create the upload URL - use web API endpoint
	entityName := fmt.Sprintf("%s_0_%d", uploadID, time.Now().Unix())
	uploadURL := fmt.Sprintf("https://www.instagram.com/rupload_igphoto/%s", entityName)

	req, err := http.NewRequest("POST", uploadURL, bytes.NewReader(photoData))
	if err != nil {
		return nil, err
	}

	// Build rupload params
	ruploadParams := map[string]interface{}{
		"upload_id":  uploadID,
		"media_type": "1", // Photo
	}
	ruploadJSON, _ := json.Marshal(ruploadParams)

	// Set web-style headers
	req.Header.Set("X-Entity-Type", contentType)
	req.Header.Set("Offset", "0")
	req.Header.Set("X-Instagram-Rupload-Params", string(ruploadJSON))
	req.Header.Set("X-Entity-Name", entityName)
	req.Header.Set("X-Entity-Length", strconv.Itoa(len(photoData)))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", strconv.Itoa(len(photoData)))

	c.setWebUploadHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Photo upload status: %d, body: %s\n", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// configurePhotoStory configures an uploaded photo as a story using web API
func (c *Client) configurePhotoStory(uploadID string) (*StoryUploadResponse, error) {
	// Build configure data as JSON
	configData := map[string]interface{}{
		"upload_id":   uploadID,
		"source_type": "3",
		"caption":     "",
	}

	jsonData, err := json.Marshal(configData)
	if err != nil {
		return nil, err
	}

	// Create request to web API endpoint
	configURL := "https://www.instagram.com/api/v1/media/configure_to_story/"

	req, err := http.NewRequest("POST", configURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	c.setWebUploadHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Photo configure response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Photo configure response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("configure failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var storyResp StoryUploadResponse
	if err := json.Unmarshal(body, &storyResp); err != nil {
		return nil, fmt.Errorf("failed to parse configure response: %w", err)
	}

	if storyResp.Status != "ok" {
		return nil, fmt.Errorf("configure failed: status=%s", storyResp.Status)
	}

	return &storyResp, nil
}
