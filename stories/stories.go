package stories

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-instagram-cli/client"
	"github.com/urfave/cli/v3"
)

// StoriesCommand is the CLI command for viewing and posting stories
var StoriesCommand = &cli.Command{
	Name:  "stories",
	Usage: "View your active stories and their stats",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Show detailed viewer information",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable debug output",
		},
	},
	Commands: []*cli.Command{
		{
			Name:      "post",
			Usage:     "Post a photo or video to your story",
			ArgsUsage: "<file>",
			Aliases:   []string{"upload", "p", "u"},
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "debug",
					Aliases: []string{"d"},
					Usage:   "Enable debug output",
				},
			},
			Action: postStoryAction,
		},
	},
	Action: storiesAction,
}

func storiesAction(ctx context.Context, cmd *cli.Command) error {
	storage, err := client.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	storedSession, err := storage.LoadSession()
	if err != nil || storedSession == nil {
		fmt.Println("❌ Not logged in")
		fmt.Println("\nPlease login first using: go-instagram-cli login")
		return nil
	}

	igClient, err := storage.RestoreClient(storedSession)
	if err != nil {
		fmt.Println("❌ Session corrupted")
		fmt.Println("\nPlease login again using: go-instagram-cli login --force")
		return nil
	}

	igClient.Debug = cmd.Bool("debug")
	verbose := cmd.Bool("verbose")

	fmt.Printf("📊 Fetching stories for @%s...\n\n", storedSession.Username)

	summary, err := igClient.GetMyStories()
	if err != nil {
		return fmt.Errorf("failed to fetch stories: %w", err)
	}

	if summary.TotalStories == 0 {
		fmt.Println("📭 You have no active stories")
		return nil
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  📸 Active Stories: %d\n", summary.TotalStories)
	fmt.Printf("  👁  Total Views:   %d\n", summary.TotalViews)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for i, story := range summary.Stories {
		icon := "📷"
		if story.MediaType == "Video" {
			icon = "🎥"
		}

		fmt.Printf("%s Story #%d (%s)\n", icon, i+1, story.MediaType)
		fmt.Printf("   ├─ Posted:    %s\n", story.PostedAt.Format("Jan 2, 3:04 PM"))
		fmt.Printf("   ├─ Expires:   %s (%s remaining)\n", story.ExpiresAt.Format("Jan 2, 3:04 PM"), story.TimeRemaining)
		fmt.Printf("   └─ Views:     %d\n", story.ViewCount)

		if verbose && len(story.Viewers) > 0 {
			fmt.Println()
			fmt.Println("   👥 Viewers:")
			maxViewers := len(story.Viewers)
			if maxViewers > 10 {
				maxViewers = 10
			}
			for j, viewer := range story.Viewers[:maxViewers] {
				verified := ""
				if viewer.IsVerified {
					verified = " ✓"
				}
				prefix := "   ├─"
				if j == maxViewers-1 {
					prefix = "   └─"
				}
				fmt.Printf("%s @%s%s", prefix, viewer.Username, verified)
				if viewer.FullName != "" {
					fmt.Printf(" (%s)", viewer.FullName)
				}
				fmt.Println()
			}
			if len(story.Viewers) > 10 {
				fmt.Printf("   ... and %d more viewers\n", story.ViewCount-len(story.Viewers[:maxViewers]))
			}
		}

		fmt.Println()
	}

	if !verbose && summary.TotalViews > 0 {
		fmt.Println("Tip: Use --verbose or -v to see who viewed your stories")
	}

	return nil
}

func postStoryAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() < 1 {
		fmt.Println("❌ Please provide a file to post")
		fmt.Println("\nUsage: go-instagram-cli stories post <file>")
		fmt.Println("Example: go-instagram-cli stories post video.mp4")
		return nil
	}

	filePath := cmd.Args().First()

	storage, err := client.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	storedSession, err := storage.LoadSession()
	if err != nil || storedSession == nil {
		fmt.Println("❌ Not logged in")
		fmt.Println("\nPlease login first using: go-instagram-cli login")
		return nil
	}

	igClient, err := storage.RestoreClient(storedSession)
	if err != nil {
		fmt.Println("❌ Session corrupted")
		fmt.Println("\nPlease login again using: go-instagram-cli login --force")
		return nil
	}

	igClient.Debug = cmd.Bool("debug")

	ext := strings.ToLower(filepath.Ext(filePath))
	isVideo := ext == ".mp4" || ext == ".mov" || ext == ".m4v" || ext == ".avi" || ext == ".mkv"
	isPhoto := ext == ".jpg" || ext == ".jpeg" || ext == ".png"

	if !isVideo && !isPhoto {
		fmt.Printf("❌ Unsupported file type: %s\n", ext)
		fmt.Println("\nSupported formats:")
		fmt.Println("  Videos: .mp4, .mov, .m4v, .avi, .mkv")
		fmt.Println("  Photos: .jpg, .jpeg, .png")
		return nil
	}

	fmt.Printf("📤 Posting story for @%s...\n\n", storedSession.Username)

	if isVideo {
		// Post video story
		result, err := igClient.UploadStory(filePath)
		if err != nil {
			return fmt.Errorf("failed to post video story: %w", err)
		}

		if result.Success {
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("  ✅ Story posted successfully!")
			if result.TotalParts > 1 {
				fmt.Printf("  📹 Video split into %d parts\n", result.TotalParts)
			}
			fmt.Printf("  🆔 Media ID: %s\n", result.MediaID)
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		} else {
			fmt.Printf("❌ Failed to post story: %v\n", result.Error)
		}
	} else {
		// Post photo story
		resp, err := igClient.PostPhotoStory(filePath)
		if err != nil {
			return fmt.Errorf("failed to post photo story: %w", err)
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ✅ Story posted successfully!")
		fmt.Printf("  🆔 Media ID: %s\n", resp.Media.ID)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	return nil
}
