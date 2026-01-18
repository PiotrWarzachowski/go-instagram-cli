package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Story represents a single story item
type Story struct {
	ID               string  `json:"id"`
	MediaType        int     `json:"media_type"` // 1 = photo, 2 = video
	TakenAt          int64   `json:"taken_at"`
	ExpiringAt       int64   `json:"expiring_at"`
	ViewerCount      int     `json:"viewer_count"`
	TotalViewerCount int     `json:"total_viewer_count"`
	ImageURL         string  `json:"image_url"`
	VideoURL         string  `json:"video_url"`
	VideoDuration    float64 `json:"video_duration"`
	Caption          string  `json:"caption"`
}

// StoryTray represents the user's story tray
type StoryTray struct {
	ID              int64   `json:"id"`
	Username        string  `json:"user"`
	LatestReelMedia int64   `json:"latest_reel_media"`
	ExpiringAt      int64   `json:"expiring_at"`
	Seen            int64   `json:"seen"`
	HasBestiesMedia bool    `json:"has_besties_media"`
	MediaCount      int     `json:"media_count"`
	Items           []Story `json:"items"`
}

// StoryViewersResponse represents the response from story viewers endpoint
type StoryViewersResponse struct {
	Users            []StoryViewer `json:"users"`
	NextMaxID        string        `json:"next_max_id"`
	TotalViewerCount int           `json:"total_viewer_count"`
	Status           string        `json:"status"`
}

// StoryViewer represents a user who viewed a story
type StoryViewer struct {
	PK            string `json:"pk"` // Instagram returns pk as string
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
	IsPrivate     bool   `json:"is_private"`
	IsVerified    bool   `json:"is_verified"`
}

// StorySummary represents a summary of story stats
type StorySummary struct {
	TotalStories int
	TotalViews   int
	Stories      []StoryStats
}

// StoryStats represents stats for a single story
type StoryStats struct {
	ID            string
	MediaType     string
	PostedAt      time.Time
	ExpiresAt     time.Time
	TimeRemaining string
	ViewCount     int
	Viewers       []StoryViewer
}

// GetMyStories fetches the current user's active stories with stats
func (c *Client) GetMyStories() (*StorySummary, error) {
	if c.UserID() == 0 || c.GetSessionID() == "" {
		return nil, fmt.Errorf("not logged in")
	}

	// Fetch user's story reel
	userID := c.UserID()
	stories, err := c.fetchUserStories(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stories: %w", err)
	}

	summary := &StorySummary{
		TotalStories: len(stories),
		Stories:      make([]StoryStats, 0, len(stories)),
	}

	// Get detailed stats for each story
	for _, story := range stories {
		stats := StoryStats{
			ID:        story.ID,
			MediaType: getMediaTypeString(story.MediaType),
			PostedAt:  time.Unix(story.TakenAt, 0),
			ExpiresAt: time.Unix(story.ExpiringAt, 0),
			ViewCount: story.TotalViewerCount,
		}

		// Calculate time remaining
		remaining := time.Until(stats.ExpiresAt)
		if remaining > 0 {
			hours := int(remaining.Hours())
			minutes := int(remaining.Minutes()) % 60
			stats.TimeRemaining = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			stats.TimeRemaining = "Expired"
		}

		// Fetch viewers for this story (this gives us accurate view count)
		viewers, totalCount, err := c.getStoryViewers(story.ID)
		if err == nil {
			stats.Viewers = viewers
			// Use the total count from viewers API if available
			if totalCount > 0 {
				stats.ViewCount = totalCount
			}
		}

		summary.TotalViews += stats.ViewCount
		summary.Stories = append(summary.Stories, stats)
	}

	return summary, nil
}

// fetchUserStories fetches stories for a given user ID
func (c *Client) fetchUserStories(userID int64) ([]Story, error) {
	url := fmt.Sprintf("https://www.instagram.com/api/v1/feed/user/%d/story/", userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setWebHeaders(req)

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
		fmt.Printf("[DEBUG] Stories response: %s\n", string(body))
	}

	var result struct {
		Reel struct {
			Items []struct {
				ID               string `json:"id"`
				Pk               string `json:"pk"` // Instagram returns pk as string
				MediaType        int    `json:"media_type"`
				TakenAt          int64  `json:"taken_at"`
				ExpiringAt       int64  `json:"expiring_at"`
				ViewerCount      int    `json:"viewer_count"`
				TotalViewerCount int    `json:"total_viewer_count"`
				// Viewer info embedded
				Viewers []struct {
					Pk       string `json:"pk"` // Instagram returns pk as string
					Username string `json:"username"`
				} `json:"viewers"`
				ViewerCursor           string `json:"viewer_cursor"`
				TotalUniqueViewerCount int    `json:"total_unique_viewer_count"`
				ImageVersions2         struct {
					Candidates []struct {
						URL string `json:"url"`
					} `json:"candidates"`
				} `json:"image_versions2"`
				VideoVersions []struct {
					URL string `json:"url"`
				} `json:"video_versions"`
				VideoDuration float64 `json:"video_duration"`
				Caption       *struct {
					Text string `json:"text"`
				} `json:"caption"`
			} `json:"items"`
			MediaCount int `json:"media_count"`
		} `json:"reel"`
		Status string `json:"status"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse stories: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("failed to fetch stories: status=%s", result.Status)
	}

	stories := make([]Story, 0, len(result.Reel.Items))
	for _, item := range result.Reel.Items {
		// Use the best available view count
		viewCount := item.TotalViewerCount
		if item.TotalUniqueViewerCount > viewCount {
			viewCount = item.TotalUniqueViewerCount
		}
		if viewCount == 0 {
			viewCount = item.ViewerCount
		}

		story := Story{
			ID:               item.ID,
			MediaType:        item.MediaType,
			TakenAt:          item.TakenAt,
			ExpiringAt:       item.ExpiringAt,
			ViewerCount:      viewCount,
			TotalViewerCount: viewCount,
			VideoDuration:    item.VideoDuration,
		}

		if len(item.ImageVersions2.Candidates) > 0 {
			story.ImageURL = item.ImageVersions2.Candidates[0].URL
		}
		if len(item.VideoVersions) > 0 {
			story.VideoURL = item.VideoVersions[0].URL
		}
		if item.Caption != nil {
			story.Caption = item.Caption.Text
		}

		stories = append(stories, story)
	}

	return stories, nil
}

// getStoryViewers fetches viewers for a specific story
// Returns: viewers list, total viewer count, error
func (c *Client) getStoryViewers(storyID string) ([]StoryViewer, int, error) {
	url := fmt.Sprintf("https://www.instagram.com/api/v1/media/%s/list_reel_media_viewer/", storyID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Story viewers response: %s\n", string(body))
	}

	var result struct {
		Users            []StoryViewer `json:"users"`
		TotalViewerCount int           `json:"total_viewer_count"`
		Status           string        `json:"status"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, err
	}

	return result.Users, result.TotalViewerCount, nil
}

// setWebHeaders sets the required headers for web API requests
func (c *Client) setWebHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.getWebUserAgent())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("X-CSRFToken", c.CSRFToken())
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-ASBD-ID", "198387")
	req.Header.Set("X-IG-WWW-Claim", c.IgWwwClaim)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Referer", "https://www.instagram.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
}

// getMediaTypeString converts media type int to string
func getMediaTypeString(mediaType int) string {
	switch mediaType {
	case 1:
		return "Photo"
	case 2:
		return "Video"
	default:
		return "Unknown"
	}
}
