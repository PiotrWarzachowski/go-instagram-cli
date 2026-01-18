package instagram

import (
	"encoding/json"
	"time"
)

type Thread struct {
	ThreadID          string        `json:"thread_id"`
	ThreadTitle       string        `json:"thread_title"`
	ThreadType        string        `json:"thread_type"`
	LastActivityAt    json.Number   `json:"last_activity_at"`
	Muted             bool          `json:"muted"`
	IsPin             bool          `json:"is_pin"`
	Named             bool          `json:"named"`
	Pending           bool          `json:"pending"`
	Users             []ThreadUser  `json:"users"`
	Items             []MessageItem `json:"items"`
	LastPermanentItem MessageItem   `json:"last_permanent_item"`
	UnseenCount       int           `json:"unseen_count"`
	HasNewer          bool          `json:"has_newer"`
	HasOlder          bool          `json:"has_older"`
	ViewerID          json.Number   `json:"viewer_id"`
	Inviter           *ThreadUser   `json:"inviter,omitempty"`
}

type ThreadUser struct {
	Pk               json.Number    `json:"pk"`
	Username         string         `json:"username"`
	FullName         string         `json:"full_name"`
	IsPrivate        bool           `json:"is_private"`
	ProfilePicURL    string         `json:"profile_pic_url"`
	ProfilePicID     string         `json:"profile_pic_id,omitempty"`
	IsVerified       bool           `json:"is_verified"`
	FriendshipStatus map[string]any `json:"friendship_status,omitempty"`
}

type MessageItem struct {
	ItemID        string      `json:"item_id"`
	UserID        json.Number `json:"user_id"`
	Timestamp     json.Number `json:"timestamp"`
	ItemType      string      `json:"item_type"`
	Text          string      `json:"text,omitempty"`
	ClientContext string      `json:"client_context,omitempty"`

	MediaShare  *MediaShare  `json:"media_share,omitempty"`
	VoiceMedia  *VoiceMedia  `json:"voice_media,omitempty"`
	VisualMedia *VisualMedia `json:"visual_media,omitempty"`
	ReelShare   *ReelShare   `json:"reel_share,omitempty"`
	StoryShare  *StoryShare  `json:"story_share,omitempty"`
	Link        *LinkShare   `json:"link,omitempty"`

	Reactions *Reactions `json:"reactions,omitempty"`

	RepliedToMessage *MessageItem `json:"replied_to_message,omitempty"`
}

type MediaShare struct {
	MediaType int         `json:"media_type"`
	ID        string      `json:"id"`
	Code      string      `json:"code"`
	User      *ThreadUser `json:"user,omitempty"`
}

type VoiceMedia struct {
	Media struct {
		ID  string `json:"id"`
		URL string `json:"audio_src"`
	} `json:"media"`
}

type VisualMedia struct {
	MediaType int    `json:"media_type"`
	URL       string `json:"url_expire_at_secs,omitempty"`
}

type ReelShare struct {
	Text     string      `json:"text,omitempty"`
	ReelType string      `json:"type,omitempty"`
	Media    *MediaShare `json:"media,omitempty"`
}

type StoryShare struct {
	Text            string      `json:"text,omitempty"`
	Media           *MediaShare `json:"media,omitempty"`
	IsReelPersisted bool        `json:"is_reel_persisted"`
}

type LinkShare struct {
	Text        string `json:"text"`
	LinkContext struct {
		LinkURL      string `json:"link_url"`
		LinkTitle    string `json:"link_title"`
		LinkSummary  string `json:"link_summary,omitempty"`
		LinkImageURL string `json:"link_image_url,omitempty"`
	} `json:"link_context"`
}

type Reactions struct {
	Likes []struct {
		SenderID      json.Number `json:"sender_id"`
		Timestamp     json.Number `json:"timestamp"`
		ClientContext string      `json:"client_context,omitempty"`
	} `json:"likes,omitempty"`
	Emojis []struct {
		SenderID  json.Number `json:"sender_id"`
		Timestamp json.Number `json:"timestamp"`
		Emoji     string      `json:"emoji"`
	} `json:"emojis,omitempty"`
}

type InboxResponse struct {
	Inbox struct {
		Threads              []Thread    `json:"threads"`
		HasOlder             bool        `json:"has_older"`
		UnseenCount          int         `json:"unseen_count"`
		UnseenCountTimestamp json.Number `json:"unseen_count_ts"`
		OldestCursor         string      `json:"oldest_cursor"`
		BlendedInboxEnabled  bool        `json:"blended_inbox_enabled"`
	} `json:"inbox"`
	SeqID                 json.Number `json:"seq_id"`
	PendingRequestsTotal  int         `json:"pending_requests_total"`
	HasPendingTopRequests bool        `json:"has_pending_top_requests"`
	Status                string      `json:"status"`
}

type ThreadResponse struct {
	Thread Thread `json:"thread"`
	Status string `json:"status"`
}

type SendMessageResponse struct {
	Action  string `json:"action"`
	Status  string `json:"status"`
	Payload struct {
		ClientContext string `json:"client_context"`
		ItemID        string `json:"item_id"`
		Timestamp     string `json:"timestamp"`
		ThreadID      string `json:"thread_id"`
	} `json:"payload"`
}

type Conversation struct {
	ThreadID      string
	Title         string
	Users         []string
	LastMessage   string
	LastMessageAt time.Time
	UnreadCount   int
	IsMuted       bool
	IsPinned      bool
}

type Message struct {
	ID          string
	SenderID    int64
	SenderName  string
	Text        string
	Type        string
	Timestamp   time.Time
	IsFromMe    bool
	HasReaction bool
}
