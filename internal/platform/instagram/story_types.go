package instagram

import "time"

type Story struct {
	ID               string  `json:"id"`
	MediaType        int     `json:"media_type"`
	TakenAt          int64   `json:"taken_at"`
	ExpiringAt       int64   `json:"expiring_at"`
	ViewerCount      int     `json:"viewer_count"`
	TotalViewerCount int     `json:"total_viewer_count"`
	ImageURL         string  `json:"image_url"`
	VideoURL         string  `json:"video_url"`
	VideoDuration    float64 `json:"video_duration"`
	Caption          string  `json:"caption"`
}

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

type StoryViewer struct {
	PK            string `json:"pk"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	ProfilePicURL string `json:"profile_pic_url"`
	IsPrivate     bool   `json:"is_private"`
	IsVerified    bool   `json:"is_verified"`
}

type StorySummary struct {
	TotalStories int
	TotalViews   int
	Stories      []StoryStats
}

type StoryStats struct {
	ID            string
	MediaType     string
	PostedAt      time.Time
	ExpiresAt     time.Time
	TimeRemaining string
	ViewCount     int
	Viewers       []StoryViewer
}

type StoryPostResult struct {
	Success     bool
	MediaID     string
	PartsPosted int
	TotalParts  int
	Errors      []error
}

type StoryUploadResponse struct {
	Media struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Pk   int64  `json:"pk"`
	} `json:"media"`
	Status string `json:"status"`
}

type StoryFeedResponse struct {
	Reel   Reel   `json:"reel"`
	Status string `json:"status"`
}

type Reel struct {
	Items      []StoryItem `json:"items"`
	MediaCount int         `json:"media_count"`
}

type StoryItem struct {
	ID                     string           `json:"id"`
	Pk                     string           `json:"pk"`
	MediaType              int              `json:"media_type"`
	TakenAt                int64            `json:"taken_at"`
	ExpiringAt             int64            `json:"expiring_at"`
	ViewerCount            int              `json:"viewer_count"`
	TotalViewerCount       int              `json:"total_viewer_count"`
	TotalUniqueViewerCount int              `json:"total_unique_viewer_count"`
	Viewers                []StoryViewerRaw `json:"viewers"`
	ViewerCursor           string           `json:"viewer_cursor"`
	ImageVersions2         ImageVersions    `json:"image_versions2"`
	VideoVersions          []VideoVersion   `json:"video_versions"`
	VideoDuration          float64          `json:"video_duration"`
	Caption                *Caption         `json:"caption"`
}

type StoryViewerRaw struct {
	Pk       string `json:"pk"`
	Username string `json:"username"`
}

type ImageVersions struct {
	Candidates []ImageCandidate `json:"candidates"`
}

type ImageCandidate struct {
	URL string `json:"url"`
}

type VideoVersion struct {
	URL string `json:"url"`
}

type Caption struct {
	Text string `json:"text"`
}

type StoryViewersResponse struct {
	Users            []StoryViewer `json:"users"`
	TotalViewerCount int           `json:"total_viewer_count"`
	Status           string        `json:"status"`
}
