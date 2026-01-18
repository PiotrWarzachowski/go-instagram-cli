package instagram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c *Client) GetInbox(cursor string, limit int) (*InboxResponse, error) {
	if limit <= 0 {
		limit = 20
	}

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

func (c *Client) GetThread(threadID string, cursor string, limit int) (*ThreadResponse, error) {
	if limit <= 0 {
		limit = 20
	}

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

func (c *Client) SendMessage(threadID string, text string) (*SendMessageResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *Client) MarkThreadSeen(threadID string, itemID string) error {
	return fmt.Errorf("not implemented")
}

func (c *Client) ApproveThread(threadID string) error {
	return fmt.Errorf("not implemented")
}

func (c *Client) DeclineThread(threadID string) error {
	return fmt.Errorf("not implemented")
}

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

		for _, user := range thread.Users {
			conv.Users = append(conv.Users, user.Username)
		}

		if conv.Title == "" && len(conv.Users) > 0 {
			conv.Title = conv.Users[0]
			if len(conv.Users) > 1 {
				conv.Title = fmt.Sprintf("%s +%d", conv.Users[0], len(conv.Users)-1)
			}
		}

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

func (c *Client) GetMessages(threadID string, limit int) ([]Message, map[int64]string, error) {
	threadResp, err := c.GetThread(threadID, "", limit)
	if err != nil {
		return nil, nil, err
	}

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
