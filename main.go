package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-instagram-cli/login"
	"github.com/go-instagram-cli/messages"
	"github.com/go-instagram-cli/stories"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:    "go-instagram-cli",
		Usage:   "Instagram CLI tool",
		Version: "1.0.0",
		Action: func(context.Context, *cli.Command) error {
			fmt.Println("Instagram CLI - Use 'go-instagram-cli help' for available commands")
			return nil
		},
		Commands: []*cli.Command{
			login.LoginCommand,
			login.LogoutCommand,
			login.StatusCommand,
			stories.StoriesCommand,
			messages.MessagesCommand,
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
