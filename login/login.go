package login

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/go-instagram-cli/client"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

// LoginCommand is the CLI command for Instagram login
var LoginCommand = &cli.Command{
	Name:  "login",
	Usage: "Login to your Instagram account",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Instagram username",
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "Instagram password (not recommended, use interactive prompt)",
		},
		&cli.StringFlag{
			Name:    "session",
			Aliases: []string{"s"},
			Usage:   "Login using session ID",
		},
		&cli.StringFlag{
			Name:  "2fa",
			Usage: "Two-factor authentication code",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Force new login even if session exists",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "Enable debug output",
		},
	},
	Action: loginAction,
}

var LogoutCommand = &cli.Command{
	Name:  "logout",
	Usage: "Logout from your Instagram account",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "clear-credentials",
			Usage: "Also delete saved username/password",
		},
	},
	Action: logoutAction,
}

var StatusCommand = &cli.Command{
	Name:   "status",
	Usage:  "Check current login status",
	Action: statusAction,
}

func loginAction(ctx context.Context, cmd *cli.Command) error {
	// Initialize session storage
	storage, err := client.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	forceLogin := cmd.Bool("force")

	// Check for existing session unless force is specified
	if !forceLogin {
		storedSession, err := storage.LoadSession()
		if err == nil && storedSession != nil {
			// Restore client from stored session
			igClient, err := storage.RestoreClient(storedSession)
			if err == nil && igClient.IsSessionValid() {
				fmt.Printf("✓ Already logged in as %s\n", storedSession.Username)
				fmt.Printf("  Session storage: %s\n", storage.GetBasePath())
				return nil
			}
		}
	}

	// Handle session ID login
	sessionID := cmd.String("session")
	if sessionID != "" {
		return loginWithSessionID(storage, sessionID)
	}

	var username string
	var password string

	savedCreds, err := storage.LoadCredentials()
	if err == nil && savedCreds != nil && savedCreds.Username != "" {
		fmt.Printf("💾 Saved credentials found for @%s\n", savedCreds.Username)
		useSaved, _ := promptInput("Use saved credentials? [Y/n]: ")
		if useSaved == "" || strings.ToLower(useSaved) == "y" || strings.ToLower(useSaved) == "yes" {
			username = savedCreds.Username
			password = savedCreds.Password
			fmt.Printf("Using saved credentials for @%s\n", username)
		}
	} else {
		username = cmd.String("username")
		password = cmd.String("password")
	}

	// Get username if still empty
	if username == "" {
		var err error
		username, err = promptInput("Username: ")
		if err != nil {
			return fmt.Errorf("failed to read username: %w", err)
		}
	}

	// Get password if still empty
	if password == "" {
		var err error
		password, err = promptPassword("Password: ")
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
	}

	// Get 2FA code if provided
	twoFactorCode := cmd.String("2fa")

	// Check debug flag
	debug := cmd.Bool("debug")

	// Create new Instagram client
	igClient := client.NewClientWithCredentials(username, password)
	igClient.Debug = debug

	fmt.Println("Logging in...")

	// Attempt login
	result, err := igClient.Login(username, password, twoFactorCode)
	if err != nil {
		// Handle 2FA requirement
		if result != nil && result.TwoFactorRequired {
			fmt.Println("\n⚠ Two-factor authentication required")

			if twoFactorCode == "" {
				twoFactorCode, err = promptInput("Enter 2FA code: ")
				if err != nil {
					return fmt.Errorf("failed to read 2FA code: %w", err)
				}
			}

			// Retry with 2FA code
			result, err = igClient.Login(username, password, twoFactorCode)
			if err != nil {
				return fmt.Errorf("2FA login failed: %w", err)
			}
		} else if result != nil && result.ChallengeRequired {
			fmt.Println("\n⚠ Instagram security challenge required")
			fmt.Println("  Please complete the challenge in the Instagram app or website")
			return fmt.Errorf("challenge required")
		} else {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	if result.Success {
		// Save session
		if err := storage.SaveSession(igClient, password); err != nil {
			fmt.Printf("⚠ Warning: Failed to save session: %v\n", err)
		}

		// Save credentials for quick re-login
		if err := storage.SaveCredentials(username, password); err != nil {
			fmt.Printf("⚠ Warning: Failed to save credentials: %v\n", err)
		}

		fmt.Printf("\n✓ Successfully logged in as %s\n", username)
		fmt.Printf("  User ID: %d\n", result.UserID)
		fmt.Printf("  Session saved to: %s\n", storage.GetBasePath())
		fmt.Println("  💾 Credentials cached for quick re-login")
	}

	return nil
}

func loginWithSessionID(storage *client.SessionStorage, sessionID string) error {
	igClient := client.NewClient()

	result, err := igClient.LoginBySessionID(sessionID)
	if err != nil {
		return fmt.Errorf("session login failed: %w", err)
	}

	if result.Success {
		// Save session (password not needed for session ID login)
		if err := storage.SaveSession(igClient, ""); err != nil {
			fmt.Printf("⚠ Warning: Failed to save session: %v\n", err)
		}

		fmt.Printf("\n✓ Successfully logged in with session ID\n")
		fmt.Printf("  User ID: %d\n", result.UserID)
		fmt.Printf("  Session saved to: %s\n", storage.GetBasePath())
	}

	return nil
}

func logoutAction(ctx context.Context, cmd *cli.Command) error {
	storage, err := client.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	clearCreds := cmd.Bool("clear-credentials")

	// Check for existing session
	storedSession, err := storage.LoadSession()
	if err != nil || storedSession == nil {
		fmt.Println("Not currently logged in")
		return nil
	}

	igClient, err := storage.RestoreClient(storedSession)
	if err != nil {
		if err := storage.DeleteSession(); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}
		fmt.Println("✓ Local session deleted")
		return nil
	}

	// Try to logout from Instagram
	if err := igClient.Logout(); err != nil {
		fmt.Printf("⚠ Warning: API logout failed: %v\n", err)
	}

	// Delete local session
	if err := storage.DeleteSession(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	storage.ClearCache()

	fmt.Printf("✓ Successfully logged out from %s\n", storedSession.Username)

	if clearCreds {
		if err := storage.DeleteCredentials(); err != nil {
			fmt.Printf("⚠ Warning: Failed to delete credentials: %v\n", err)
		} else {
			fmt.Println("  Saved credentials deleted")
		}
	} else if storage.HasCredentials() {
		fmt.Println("  💾 Credentials still saved for quick re-login")
		fmt.Println("     Use 'logout --clear-credentials' to remove them")
	}

	return nil
}

func statusAction(ctx context.Context, cmd *cli.Command) error {
	storage, err := client.NewSessionStorage()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	storedSession, err := storage.LoadSession()
	if err != nil || storedSession == nil {
		fmt.Println("Status: Not logged in")
		fmt.Println("\nUse 'go-instagram-cli login' to authenticate")
		return nil
	}

	igClient, err := storage.RestoreClient(storedSession)
	if err != nil {
		fmt.Println("Status: Session corrupted")
		fmt.Println("\nUse 'go-instagram-cli login --force' to create a new session")
		return nil
	}

	fmt.Println("Status: Logged in")
	fmt.Printf("  Username: %s\n", storedSession.Username)

	if igClient.UserID() != 0 {
		fmt.Printf("  User ID: %d\n", igClient.UserID())
	}

	if igClient.IsSessionValid() {
		fmt.Println("  Session: Valid")
	} else {
		fmt.Println("  Session: Expired (will attempt refresh on next request)")
	}

	fmt.Printf("  Storage: %s\n", storage.GetBasePath())

	return nil
}

// promptInput prompts for user input
func promptInput(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// promptPassword prompts for password input (hidden)
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Try to read password securely
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after password input
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	// Fallback to regular input if not a terminal
	return promptInput("")
}
