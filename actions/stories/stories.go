package stories

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/go-instagram-cli/internal/platform/instagram"
	"github.com/go-instagram-cli/internal/storage"
	"github.com/go-instagram-cli/providers"
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
	storage, err := storage.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	storedSession, err := storage.LoadSession()
	if err != nil || storedSession == nil {
		fmt.Println("❌ Not logged in")
		fmt.Println("\nPlease login first using: go-instagram-cli login")
		return nil
	}

	igClient, err := instagram.NewClientFromSession(storedSession)
	if err != nil {
		fmt.Println("❌ Session corrupted")
		fmt.Println("\nPlease login again using: go-instagram-cli login --force")
		return nil
	}

	igClient.Debug = cmd.Bool("debug")
	verbose := cmd.Bool("verbose")

	fmt.Printf("📊 Fetching stories for @%s...\n\n", storedSession.Username)

	summary, err := igClient.GetMyStories(ctx)
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

// actions/stories/post.go

func postStoryAction(ctx context.Context, cmd *cli.Command) error {
	videoPath := cmd.Args().First()

	// Initialize your provider (assuming you have a setup helper)
	provider, err := providers.NewStoryProvider()
	if err != nil {
		return err
	}

	// Create the UI observer
	reporter := NewCLIReporter()

	result, err := provider.UploadWithProgress(ctx, videoPath, reporter)

	reporter.Wait()

	if err != nil {
		fmt.Printf("\n❌ Critical Error: %v\n", err)
		return nil
	}

	// Final Summary
	if result.Success {
		fmt.Printf("\n✅ Successfully posted %d/%d segments!\n", result.PartsPosted, result.TotalParts)
	} else {
		fmt.Printf("\n⚠️ Completed with errors. Posted %d/%d segments.\n", result.PartsPosted, result.TotalParts)
		for _, e := range result.Errors {
			fmt.Printf("  - %v\n", e)
		}
	}

	return nil
}
