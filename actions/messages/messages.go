package messages

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/go-instagram-cli/internal/platform/instagram"
	"github.com/go-instagram-cli/internal/storage"
)

const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorBgBlue  = "\033[44m"
	colorBgGray  = "\033[100m"
)

var MessagesCommand = &cli.Command{
	Name:    "messages",
	Aliases: []string{"dm", "inbox", "dms"},
	Usage:   "View and manage your Instagram direct messages",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable debug mode",
		},
	},
	Action: messagesAction,
}

type conversationCache struct {
	conversations []instagram.Conversation
	lastRefresh   time.Time
}

var cache = &conversationCache{}

func messagesAction(ctx context.Context, cmd *cli.Command) error {
	debug := cmd.Bool("debug")

	storage, err := storage.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	stored, err := storage.LoadSession()
	if err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	if stored == nil {
		fmt.Printf("%s✗ Not logged in. Please run 'go-instagram-cli login' first.%s\n", colorRed, colorReset)
		return nil
	}

	c, err := instagram.NewClientFromSession(stored)
	if err != nil {
		return fmt.Errorf("failed to restore session: %w", err)
	}
	c.Debug = debug

	return runInteractiveMode(c, storage)
}

func runInteractiveMode(c *instagram.Client, storage *storage.Storage) error {
	reader := bufio.NewReader(os.Stdin)

	clearScreen()

	conversations, fromCache := getConversationsWithCache(c, storage, false)

	for {
		displayConversations(conversations)

		if fromCache && !cache.lastRefresh.IsZero() {
			ago := time.Since(cache.lastRefresh).Round(time.Second)
			fmt.Printf("%s  📋 Cached %s ago (auto-refreshes every 60s)%s\n", colorDim, ago, colorReset)
		}

		fmt.Printf("\n%s─────────────────────────────────────────────────────────%s\n", colorDim, colorReset)
		fmt.Printf("%sCommands:%s [number] View conversation • %sr%s Refresh • %sq%s Quit\n",
			colorCyan, colorReset, colorGreen, colorReset, colorRed, colorReset)
		fmt.Printf("%s➜ %s", colorGreen, colorReset)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch strings.ToLower(input) {
		case "q", "quit", "exit":
			fmt.Printf("\n%s👋 Goodbye!%s\n", colorCyan, colorReset)
			return nil
		case "r", "refresh":
			clearScreen()
			fmt.Printf("%s🔄 Refreshing...%s\n", colorCyan, colorReset)
			conversations, fromCache = getConversationsWithCache(c, storage, true)
			clearScreen()
			continue
		case "":
			if !cache.lastRefresh.IsZero() && time.Since(cache.lastRefresh) > 60*time.Second {
				conversations, fromCache = getConversationsWithCache(c, storage, true)
			}
			continue
		default:
			num, err := strconv.Atoi(input)
			if err != nil || num < 1 || num > len(conversations) {
				fmt.Printf("%s✗ Invalid selection. Enter a number 1-%d%s\n", colorRed, len(conversations), colorReset)
				time.Sleep(1 * time.Second)
				clearScreen()
				continue
			}

			conv := conversations[num-1]
			if err := openConversation(c, conv, reader); err != nil {
				fmt.Printf("%s✗ Error: %v%s\n", colorRed, err, colorReset)
				time.Sleep(2 * time.Second)
			}

			clearScreen()
			conversations, fromCache = getConversationsWithCache(c, storage, false)
		}
	}
}

func getConversationsWithCache(c *instagram.Client, storage *storage.Storage, forceRefresh bool) ([]instagram.Conversation, bool) {
	if !forceRefresh && cache.conversations != nil && !cache.lastRefresh.IsZero() {
		if time.Since(cache.lastRefresh) < 60*time.Second {
			return cache.conversations, true
		}
	}

	conversations, err := c.GetConversations()
	if err != nil {
		if cache.conversations != nil {
			fmt.Printf("%s⚠ Using cached data (fetch failed: %v)%s\n", colorYellow, err, colorReset)
			return cache.conversations, true
		}
		fmt.Printf("%s✗ Failed to fetch inbox: %v%s\n", colorRed, err, colorReset)
		return nil, false
	}

	cache.conversations = conversations
	cache.lastRefresh = time.Now()

	return conversations, false
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func displayConversations(conversations []instagram.Conversation) {
	if len(conversations) == 0 {
		fmt.Printf("\n%s📭 No conversations found.%s\n", colorDim, colorReset)
		return
	}

	fmt.Printf("\n%s%-4s %-25s %-35s %s%s\n", colorBold, "#", "FROM", "LAST MESSAGE", "TIME", colorReset)
	fmt.Printf("%s────────────────────────────────────────────────────────────────────────────%s\n", colorDim, colorReset)

	for i, conv := range conversations {
		num := fmt.Sprintf("%d", i+1)

		title := truncateString(conv.Title, 23)

		preview := truncateString(conv.LastMessage, 33)
		if preview == "" {
			preview = "[No messages]"
		}

		timeStr := formatTimeAgo(conv.LastMessageAt)

		numColor := colorDim
		titleColor := colorWhite
		previewColor := colorDim

		if conv.UnreadCount > 0 {
			numColor = colorGreen
			titleColor = colorBold + colorWhite
			previewColor = colorWhite
		}

		indicators := ""
		if conv.IsPinned {
			indicators += "📌"
		}
		if conv.IsMuted {
			indicators += "🔇"
		}
		if conv.UnreadCount > 0 {
			indicators += fmt.Sprintf(" %s(%d)%s", colorGreen, conv.UnreadCount, colorReset)
		}

		fmt.Printf("%s%-4s%s %s%-25s%s %s%-35s%s %s%s%s %s\n",
			numColor, num, colorReset,
			titleColor, title, colorReset,
			previewColor, preview, colorReset,
			colorDim, timeStr, colorReset,
			indicators,
		)
	}
}

func openConversation(c *instagram.Client, conv instagram.Conversation, reader *bufio.Reader) error {
	clearScreen()

	for {
		// Fetch messages
		messages, _, err := c.GetMessages(conv.ThreadID, 30)
		if err != nil {
			return fmt.Errorf("failed to fetch messages: %w", err)
		}

		fmt.Printf("%s%s", colorBold, colorMagenta)
		fmt.Println("╔════════════════════════════════════════════════════════════╗")
		fmt.Printf("║  💬 Conversation with: %-36s ║\n", truncateString(conv.Title, 35))
		fmt.Println("╚════════════════════════════════════════════════════════════╝")
		fmt.Printf("%s\n", colorReset)

		displayMessages(messages, c.UserID())

		fmt.Printf("\n%s─────────────────────────────────────────────────────────%s\n", colorDim, colorReset)
		fmt.Printf("%sCommands:%s Type message to reply • %sr%s Refresh • %sb%s Back\n",
			colorCyan, colorReset, colorGreen, colorReset, colorYellow, colorReset)
		fmt.Printf("%s%s ➜ %s", colorBold, conv.Title, colorReset)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch strings.ToLower(input) {
		case "b", "back":
			return nil
		case "r", "refresh":
			clearScreen()
			continue
		case "":
			continue
		default:
			// Send message
			fmt.Printf("%sSending...%s", colorDim, colorReset)
			_, err := c.SendMessage(conv.ThreadID, input)
			if err != nil {
				fmt.Printf("\r%s✗ Failed to send: %v%s\n", colorRed, err, colorReset)
				time.Sleep(2 * time.Second)
			} else {
				fmt.Printf("\r%s✓ Message sent!%s    \n", colorGreen, colorReset)
				time.Sleep(500 * time.Millisecond)
			}
			clearScreen()
		}
	}
}

func displayMessages(messages []instagram.Message, myUserID int64) {
	if len(messages) == 0 {
		fmt.Printf("\n%s📭 No messages in this conversation.%s\n", colorDim, colorReset)
		return
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	// Group messages by date
	var lastDate string

	for _, msg := range messages {
		// Date separator
		date := msg.Timestamp.Format("Mon, Jan 2 2006")
		if date != lastDate {
			fmt.Printf("\n%s──────────── %s ────────────%s\n\n", colorDim, date, colorReset)
			lastDate = date
		}

		timeStr := msg.Timestamp.Format("15:04")

		if msg.IsFromMe {
			// Right-aligned (my messages)
			displayMyMessage(msg, timeStr)
		} else {
			// Left-aligned (their messages)
			displayTheirMessage(msg, timeStr)
		}
	}
}

func displayMyMessage(msg instagram.Message, timeStr string) {
	// Format message text with wrapping
	lines := wrapText(msg.Text, 45)

	padding := strings.Repeat(" ", 15)

	for i, line := range lines {
		if i == 0 {
			fmt.Printf("%s%s%s%s%s%s %s%s%s\n",
				padding,
				colorBgBlue, colorBold, colorWhite, " "+line+" ", colorReset,
				colorWhite, timeStr, colorReset,
			)
		} else {

			fmt.Printf("%s%s%s%s%s%s\n",
				padding,
				colorBgBlue, colorBold, colorWhite, " "+line+" ", colorReset,
			)
		}
	}

	if msg.HasReaction {
		fmt.Printf("%s%s💗%s\n", padding, colorDim, colorReset)
	}
}

func displayTheirMessage(msg instagram.Message, timeStr string) {
	// Format message text with wrapping
	lines := wrapText(msg.Text, 45)

	for i, line := range lines {
		if i == 0 {
			// First line with sender name and timestamp
			fmt.Printf("%s%s%s %s%s%s %s%s%s\n",
				colorCyan, msg.SenderName, colorReset,
				colorBgGray, " "+line+" ", colorReset,
				colorDim, timeStr, colorReset,
			)
		} else {
			senderPadding := strings.Repeat(" ", len(msg.SenderName)+1)
			fmt.Printf("%s%s%s%s%s\n",
				senderPadding,
				colorBgGray, " "+line+" ", colorReset, "",
			)
		}
	}

	if msg.HasReaction {
		fmt.Printf("  %s💗%s\n", colorDim, colorReset)
	}
}

func wrapText(text string, maxWidth int) []string {
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 <= maxWidth {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			// Handle very long words
			if len(word) > maxWidth {
				for len(word) > maxWidth {
					lines = append(lines, word[:maxWidth])
					word = word[maxWidth:]
				}
				currentLine = word
			} else {
				currentLine = word
			}
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		return fmt.Sprintf("%dw", weeks)
	default:
		return t.Format("Jan 2")
	}
}
