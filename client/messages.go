package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Thread represents a DM conversation thread
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

// ThreadUser represents a user in a DM thread
type ThreadUser struct {
	Pk               json.Number    `json:"pk"` // Instagram returns as string sometimes
	Username         string         `json:"username"`
	FullName         string         `json:"full_name"`
	IsPrivate        bool           `json:"is_private"`
	ProfilePicURL    string         `json:"profile_pic_url"`
	ProfilePicID     string         `json:"profile_pic_id,omitempty"`
	IsVerified       bool           `json:"is_verified"`
	FriendshipStatus map[string]any `json:"friendship_status,omitempty"`
}

// MessageItem represents a single message in a thread
type MessageItem struct {
	ItemID        string      `json:"item_id"`
	UserID        json.Number `json:"user_id"`   // Instagram returns as string sometimes
	Timestamp     json.Number `json:"timestamp"` // Instagram returns as string sometimes
	ItemType      string      `json:"item_type"`
	Text          string      `json:"text,omitempty"`
	ClientContext string      `json:"client_context,omitempty"`

	// Media content
	MediaShare  *MediaShare  `json:"media_share,omitempty"`
	VoiceMedia  *VoiceMedia  `json:"voice_media,omitempty"`
	VisualMedia *VisualMedia `json:"visual_media,omitempty"`
	ReelShare   *ReelShare   `json:"reel_share,omitempty"`
	StoryShare  *StoryShare  `json:"story_share,omitempty"`
	Link        *LinkShare   `json:"link,omitempty"`

	// Reactions
	Reactions *Reactions `json:"reactions,omitempty"`

	// Reply info
	RepliedToMessage *MessageItem `json:"replied_to_message,omitempty"`
}

// MediaShare represents shared media in DM
type MediaShare struct {
	MediaType int         `json:"media_type"`
	ID        string      `json:"id"`
	Code      string      `json:"code"`
	User      *ThreadUser `json:"user,omitempty"`
}

// VoiceMedia represents a voice message
type VoiceMedia struct {
	Media struct {
		ID  string `json:"id"`
		URL string `json:"audio_src"`
	} `json:"media"`
}

// VisualMedia represents visual media (image/video)
type VisualMedia struct {
	MediaType int    `json:"media_type"`
	URL       string `json:"url_expire_at_secs,omitempty"`
}

// ReelShare represents a shared reel
type ReelShare struct {
	Text     string      `json:"text,omitempty"`
	ReelType string      `json:"type,omitempty"`
	Media    *MediaShare `json:"media,omitempty"`
}

// StoryShare represents a shared story
type StoryShare struct {
	Text            string      `json:"text,omitempty"`
	Media           *MediaShare `json:"media,omitempty"`
	IsReelPersisted bool        `json:"is_reel_persisted"`
}

// LinkShare represents a shared link
type LinkShare struct {
	Text        string `json:"text"`
	LinkContext struct {
		LinkURL      string `json:"link_url"`
		LinkTitle    string `json:"link_title"`
		LinkSummary  string `json:"link_summary,omitempty"`
		LinkImageURL string `json:"link_image_url,omitempty"`
	} `json:"link_context"`
}

// Reactions represents message reactions
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

// InboxResponse represents the inbox API response
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

// ThreadResponse represents a single thread API response
type ThreadResponse struct {
	Thread Thread `json:"thread"`
	Status string `json:"status"`
}

// SendMessageResponse represents the response from sending a message
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

// Conversation represents a simplified conversation for CLI display
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

// Message represents a simplified message for CLI display
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

// GetInbox fetches the user's DM inbox using the web API
func (c *Client) GetInbox(cursor string, limit int) (*InboxResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	// Use web API like stories do - this works!
	url := fmt.Sprintf("https://www.instagram.com/api/v1/direct_v2/inbox/?limit=%d&thread_message_limit=10&persistentBadging=true&folder=", limit)

	if cursor != "" {
		url += "&cursor=" + cursor
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Inbox response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Inbox response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch inbox: status %d", resp.StatusCode)
	}

	var inboxResp InboxResponse
	if err := json.Unmarshal(body, &inboxResp); err != nil {
		return nil, fmt.Errorf("failed to parse inbox response: %w", err)
	}

	return &inboxResp, nil
}

// GetThread fetches a specific thread with messages using the web API
func (c *Client) GetThread(threadID string, cursor string, limit int) (*ThreadResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	// Use web API like stories do
	url := fmt.Sprintf("https://www.instagram.com/api/v1/direct_v2/threads/%s/?limit=%d&direction=older", threadID, limit)

	if cursor != "" {
		url += "&cursor=" + cursor
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Thread response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Thread response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch thread: status %d", resp.StatusCode)
	}

	var threadResp ThreadResponse
	if err := json.Unmarshal(body, &threadResp); err != nil {
		return nil, fmt.Errorf("failed to parse thread response: %w", err)
	}

	return &threadResp, nil
}

// SendMessage sends a text message to a thread
// NOTE: Currently disabled - Instagram's web/mobile API for sending DMs causes session invalidation
// The browser uses WebSocket/MQTT (Lightspeed protocol) with WASM encoding which is complex to replicate
func (c *Client) SendMessage(threadID string, text string) (*SendMessageResponse, error) {
	return nil, fmt.Errorf("sending DMs is currently disabled to prevent session invalidation. Please use the Instagram app to reply")
}

// SendMessageViaWebAPI sends a message using Instagram's web API (like the browser does)
func (c *Client) SendMessageViaWebAPI(threadID string, text string) (*SendMessageResponse, error) {
	// Build the request URL - use web API broadcast endpoint (matches instagrapi mobile API)
	urlStr := "https://www.instagram.com/api/v1/direct_v2/threads/broadcast/text/"

	// Build form data (matching instagrapi/mobile behavior)
	clientContext := c.generateUUID()
	formData := url.Values{}
	formData.Set("thread_ids", fmt.Sprintf("[%s]", threadID))
	formData.Set("text", text)
	formData.Set("action", "send_item")
	formData.Set("client_context", clientContext)
	formData.Set("_uuid", c.UUID)
	formData.Set("mutation_token", clientContext)
	formData.Set("offline_threading_id", clientContext)
	formData.Set("device_id", c.AndroidDeviceID)

	req, err := http.NewRequest("POST", urlStr, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set web headers
	c.setWebHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.Debug {
		fmt.Printf("[DEBUG] Sending message to thread %s\n", threadID)
		fmt.Printf("[DEBUG] Request URL: %s\n", urlStr)
		fmt.Printf("[DEBUG] Form data: %s\n", formData.Encode())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Send response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Send response: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to send message: status %d, body: %s", resp.StatusCode, string(body))
	}

	var sendResp SendMessageResponse
	if err := json.Unmarshal(body, &sendResp); err != nil {
		// Try to parse as generic response
		var genericResp map[string]any
		if json.Unmarshal(body, &genericResp) == nil {
			if status, ok := genericResp["status"].(string); ok && status == "ok" {
				return &SendMessageResponse{
					Status: "ok",
					Action: "send_item",
					Payload: struct {
						ClientContext string `json:"client_context"`
						ItemID        string `json:"item_id"`
						Timestamp     string `json:"timestamp"`
						ThreadID      string `json:"thread_id"`
					}{
						ClientContext: clientContext,
						ThreadID:      threadID,
						Timestamp:     fmt.Sprintf("%d", time.Now().UnixMicro()),
					},
				}, nil
			}
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &sendResp, nil
}

// SendMessageToUser sends a text message to a user by their user ID
func (c *Client) SendMessageToUser(userID int64, text string) (*SendMessageResponse, error) {
	clientContext := c.generateUUID()

	data := c.withExtraData(map[string]any{
		"action":               "send_item",
		"recipient_users":      fmt.Sprintf("[[%d]]", userID),
		"client_context":       clientContext,
		"text":                 text,
		"device_id":            c.AndroidDeviceID,
		"mutation_token":       c.generateMutationToken(),
		"offline_threading_id": clientContext,
	})

	resp, err := c.privateRequest("direct_v2/threads/broadcast/text/", data, false)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	var sendResp SendMessageResponse
	if err := json.Unmarshal(resp.RawBody, &sendResp); err != nil {
		return nil, fmt.Errorf("failed to parse send response: %w", err)
	}

	return &sendResp, nil
}

// MarkThreadSeen marks a thread as seen
func (c *Client) MarkThreadSeen(threadID string, itemID string) error {
	data := c.withDefaultData(map[string]any{
		"thread_id":         threadID,
		"item_id":           itemID,
		"action":            "mark_seen",
		"use_unified_inbox": "true",
	})

	endpoint := fmt.Sprintf("direct_v2/threads/%s/items/%s/seen/", threadID, itemID)
	_, err := c.privateRequest(endpoint, data, false)
	if err != nil {
		return fmt.Errorf("failed to mark thread seen: %w", err)
	}

	return nil
}

// GetPendingInbox fetches pending message requests using the web API
func (c *Client) GetPendingInbox(cursor string, limit int) (*InboxResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	// Use web API
	url := fmt.Sprintf("https://www.instagram.com/api/v1/direct_v2/pending_inbox/?limit=%d&persistentBadging=true", limit)

	if cursor != "" {
		url += "&cursor=" + cursor
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setWebHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Debug {
		fmt.Printf("[DEBUG] Pending inbox response: %s\n", string(body))
	}

	var inboxResp InboxResponse
	if err := json.Unmarshal(body, &inboxResp); err != nil {
		return nil, fmt.Errorf("failed to parse pending inbox response: %w", err)
	}

	return &inboxResp, nil
}

// ApproveThread approves a pending message request
func (c *Client) ApproveThread(threadID string) error {
	// Match Python implementation
	data := map[string]any{
		"filter": "DEFAULT",
		"_uuid":  c.UUID,
	}

	endpoint := fmt.Sprintf("direct_v2/threads/%s/approve/", threadID)
	_, err := c.privateRequest(endpoint, data, false)
	if err != nil {
		return fmt.Errorf("failed to approve thread: %w", err)
	}

	return nil
}

// DeclineThread declines a pending message request
func (c *Client) DeclineThread(threadID string) error {
	data := c.withDefaultData(map[string]any{})

	endpoint := fmt.Sprintf("direct_v2/threads/%s/decline/", threadID)
	_, err := c.privateRequest(endpoint, data, false)
	if err != nil {
		return fmt.Errorf("failed to decline thread: %w", err)
	}

	return nil
}

// generateMutationToken generates a token for message mutations
func (c *Client) generateMutationToken() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// GetConversations returns a simplified list of conversations for CLI display
func (c *Client) GetConversations() ([]Conversation, error) {
	inbox, err := c.GetInbox("", 50)
	if err != nil {
		return nil, err
	}

	var conversations []Conversation
	for _, thread := range inbox.Inbox.Threads {
		conv := Conversation{
			ThreadID:    thread.ThreadID,
			Title:       thread.ThreadTitle,
			UnreadCount: thread.UnseenCount,
			IsMuted:     thread.Muted,
			IsPinned:    thread.IsPin,
		}

		// Get usernames
		for _, user := range thread.Users {
			conv.Users = append(conv.Users, user.Username)
		}

		// Set title from usernames if not set
		if conv.Title == "" && len(conv.Users) > 0 {
			conv.Title = conv.Users[0]
			if len(conv.Users) > 1 {
				conv.Title = fmt.Sprintf("%s +%d", conv.Users[0], len(conv.Users)-1)
			}
		}

		// Get last message
		if thread.LastPermanentItem.ItemType != "" {
			conv.LastMessage = formatMessagePreview(thread.LastPermanentItem)
			if ts, err := thread.LastPermanentItem.Timestamp.Int64(); err == nil {
				conv.LastMessageAt = time.Unix(0, ts*1000)
			}
		}

		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// GetMessages returns messages from a specific thread for CLI display
func (c *Client) GetMessages(threadID string, limit int) ([]Message, map[int64]string, error) {
	threadResp, err := c.GetThread(threadID, "", limit)
	if err != nil {
		return nil, nil, err
	}

	// Build user map for names
	userMap := make(map[int64]string)
	for _, user := range threadResp.Thread.Users {
		pk, _ := user.Pk.Int64()
		userMap[pk] = user.Username
	}
	userMap[c.UserID()] = "You"

	var messages []Message
	for _, item := range threadResp.Thread.Items {
		senderID, _ := item.UserID.Int64()
		ts, _ := item.Timestamp.Int64()
		msg := Message{
			ID:        item.ItemID,
			SenderID:  senderID,
			Text:      formatMessageContent(item),
			Type:      item.ItemType,
			Timestamp: time.Unix(0, ts*1000),
			IsFromMe:  senderID == c.UserID(),
		}

		if name, ok := userMap[senderID]; ok {
			msg.SenderName = name
		} else {
			msg.SenderName = fmt.Sprintf("User %d", senderID)
		}

		if item.Reactions != nil && (len(item.Reactions.Likes) > 0 || len(item.Reactions.Emojis) > 0) {
			msg.HasReaction = true
		}

		messages = append(messages, msg)
	}

	return messages, userMap, nil
}

// formatMessagePreview formats a message for preview in the conversation list
func formatMessagePreview(item MessageItem) string {
	switch item.ItemType {
	case "text":
		if len(item.Text) > 50 {
			return item.Text[:47] + "..."
		}
		return item.Text
	case "media_share":
		return "📷 Shared a post"
	case "voice_media":
		return "🎤 Voice message"
	case "visual_media":
		return "📸 Photo/Video"
	case "reel_share":
		return "🎬 Shared a reel"
	case "story_share":
		return "📖 Shared a story"
	case "link":
		return "🔗 Shared a link"
	case "like":
		return "❤️ Liked a message"
	case "animated_media":
		return "🎭 GIF"
	default:
		return fmt.Sprintf("[%s]", item.ItemType)
	}
}

// formatMessageContent formats the full message content
func formatMessageContent(item MessageItem) string {
	switch item.ItemType {
	case "text":
		return item.Text
	case "media_share":
		return "📷 [Shared a post]"
	case "voice_media":
		return "🎤 [Voice message]"
	case "visual_media":
		return "📸 [Photo/Video]"
	case "reel_share":
		if item.ReelShare != nil && item.ReelShare.Text != "" {
			return fmt.Sprintf("🎬 [Shared a reel] %s", item.ReelShare.Text)
		}
		return "🎬 [Shared a reel]"
	case "story_share":
		if item.StoryShare != nil && item.StoryShare.Text != "" {
			return fmt.Sprintf("📖 [Shared a story] %s", item.StoryShare.Text)
		}
		return "📖 [Shared a story]"
	case "link":
		if item.Link != nil {
			return fmt.Sprintf("🔗 %s", item.Link.LinkContext.LinkURL)
		}
		return "🔗 [Shared a link]"
	case "like":
		return "❤️"
	case "animated_media":
		return "🎭 [GIF]"
	case "placeholder":
		return "[Message unavailable]"
	default:
		if item.Text != "" {
			return item.Text
		}
		return fmt.Sprintf("[%s]", item.ItemType)
	}
}
