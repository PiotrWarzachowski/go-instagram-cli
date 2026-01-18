package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
	Path      string
	Duration  float64
	Width     int
	Height    int
	Codec     string
	Thumbnail string
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

func PrepareVideo(inputPath string) ([]VideoInfo, error) {
	tmpDir, _ := os.MkdirTemp("", "story_upload")
	outputPattern := filepath.Join(tmpDir, "segment_%03d.mp4")

	// 1. Process and Segment
	cmd := exec.Command("ffmpeg", "-i", inputPath,
		"-vf", "scale=1080:1920:force_original_aspect_ratio=decrease,pad=1080:1920:(1080-iw)/2:(1920-ih)/2:black",
		"-c:v", "libx264", "-crf", "23", "-preset", "veryfast",
		"-c:a", "aac", "-f", "segment", "-segment_time", "58",
		"-reset_timestamps", "1", outputPattern)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg split failed: %v", err)
	}

	// 2. Analyze segments and generate thumbnails
	segments, _ := filepath.Glob(filepath.Join(tmpDir, "segment_*.mp4"))
	var processed []VideoInfo

	for _, seg := range segments {
		thumb := seg + ".jpg"
		// Generate thumbnail at 0.5s mark
		exec.Command("ffmpeg", "-i", seg, "-ss", "00:00:00.5", "-vframes", "1", thumb).Run()

		processed = append(processed, VideoInfo{
			Path:      seg,
			Width:     1080,
			Height:    1920,
			Duration:  58.0,
			Thumbnail: thumb,
		})
	}
	return processed, nil
}

func (c *Client) RuploadVideo(info VideoInfo) (string, error) {
	uploadID := strconv.FormatInt(time.Now().UnixMilli(), 10)
	waterfallID := uuid.New().String()
	// Matches the Python: {upload_id}_0_{rand}
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

	// --- STEP 1: THE GET HANDSHAKE ---
	// This "pings" the server to prepare it for the video stream
	url := fmt.Sprintf("https://i.instagram.com/rupload_igvideo/%s", uploadName)

	getReq, _ := http.NewRequest("GET", url, nil)
	getReq.Header.Set("X-Instagram-Rupload-Params", string(paramsJSON))
	getReq.Header.Set("X_FB_VIDEO_WATERFALL_ID", waterfallID)
	getReq.Header.Set("Accept-Encoding", "gzip, deflate")
	c.setWebUploadHeaders(getReq)

	getResp, err := c.httpClient.Do(getReq)
	if err != nil {
		return "", fmt.Errorf("network error on GET: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != 200 {
		body, _ := io.ReadAll(getResp.Body)
		return "", fmt.Errorf("handshake failed (Status %d): %s", getResp.StatusCode, string(body))
	}

	// --- STEP 2: THE POST UPLOAD ---
	videoData, err := os.ReadFile(info.Path)
	if err != nil {
		return "", fmt.Errorf("could not read segment file: %v", err)
	}

	postReq, _ := http.NewRequest("POST", url, bytes.NewReader(videoData))

	// These headers are MANDATORY for the rupload protocol
	postReq.Header.Set("X-Entity-Name", uploadName)
	postReq.Header.Set("X-Entity-Length", strconv.Itoa(len(videoData)))
	postReq.Header.Set("X-Entity-Type", "video/mp4")
	postReq.Header.Set("Offset", "0")
	postReq.Header.Set("Content-Type", "application/octet-stream")
	postReq.Header.Set("X-Instagram-Rupload-Params", string(paramsJSON))
	postReq.Header.Set("X_FB_VIDEO_WATERFALL_ID", waterfallID)
	// postReq.Header.Set("Cookie", "sessionid=" + c.SessionID)

	postResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("network error on POST: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != 200 {
		body, _ := io.ReadAll(postResp.Body)
		return "", fmt.Errorf("byte upload failed (Status %d): %s", postResp.StatusCode, string(body))
	}

	return uploadID, nil
}

func (c *Client) ConfigureStory(uploadID string, info VideoInfo) error {
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

	for attempt := 0; attempt < 15; attempt++ {
		req, _ := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
		c.setMobileHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		// Handle Success
		if resp.StatusCode == 200 {
			return nil
		}

		// Handle "Transcode not finished" (Status 202)
		if strings.Contains(string(body), "Transcode not finished yet") {
			fmt.Printf("⏳ Transcoding segment %s... (Attempt %d)\n", uploadID, attempt+1)
			time.Sleep(7 * time.Second)
			continue
		}

		return fmt.Errorf("configure failed: %s", string(body))
	}

	return fmt.Errorf("timeout waiting for transcode")
}

func (c *Client) setMobileHeaders(req *http.Request) {
	// Use a standard Android Instagram User-Agent
	req.Header.Set("User-Agent", "Instagram 312.1.0.34.111 (Linux; Android 10; SM-G973F; 29/10; en_US; st_v2)")
	req.Header.Set("X-IG-App-ID", "1217981644879628") // The actual Android App ID
	req.Header.Set("X-IG-Capabilities", "3brTvw==")
	req.Header.Set("X-IG-Connection-Type", "WIFI")
	req.Header.Set("X-CSRFToken", c.Cookies["csrftoken"]) // Pull from your cookie map
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Construct Cookie header from your map
	var cookieStrings []string
	for name, value := range c.Cookies {
		cookieStrings = append(cookieStrings, fmt.Sprintf("%s=%s", name, value))
	}
	req.Header.Set("Cookie", strings.Join(cookieStrings, "; "))
}

func (c *Client) UploadStory(videoPath string) (*StoryPostResult, error) {
	fmt.Println("🚀 Processing video for Story...")
	segments, err := PrepareVideo(videoPath)
	if err != nil {
		log.Fatalf("Process error: %v", err)
	}

	for i, seg := range segments {
		fmt.Printf("📦 Uploading Segment %d/%d...\n", i+1, len(segments))

		uploadID, err := c.RuploadVideo(seg)
		if err != nil {
			log.Printf("Upload failed: %v", err)
			continue
		}

		err = c.ConfigureStory(uploadID, seg)
		if err != nil {
			log.Printf("Config failed: %v", err)
		} else {
			fmt.Println("✅ Story segment posted!")
		}
	}
	return &StoryPostResult{
		Success:     true,
		MediaID:     "123",
		PartsPosted: len(segments),
		TotalParts:  len(segments),
	}, nil
}
