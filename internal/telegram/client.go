package telegram

import (
	"context"
	"fmt"
	"io"

	"cli-tg-chat-summary/internal/config"

	"github.com/gotd/td/telegram"
)

type Client struct {
	cfg    *config.Config
	client *telegram.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	client := telegram.NewClient(cfg.TelegramAppID, cfg.TelegramAppHash, telegram.Options{})
	return &Client{
		cfg:    cfg,
		client: client,
	}, nil
}

func (c *Client) Login(input io.Reader) error {
	// We need a context for the login flow
	ctx := context.Background()

	return c.client.Run(ctx, func(ctx context.Context) error {
		// This is a simplified placeholder.
		// Real authentication flow requires handling the phone code prompt.
		// For a CLI, we likely want to start the authentication flow here
		// if not already authorized.

		status, err := c.client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get auth status: %w", err)
		}

		if !status.Authorized {
			fmt.Println("User not authorized. Please implement interactive login flow.")
			// In a real implementation:
			// flow := auth.NewFlow(term.NewAuth(input), auth.SendCodeOptions{PhoneNumber: c.cfg.Phone})
			// if err := c.client.Auth().IfNecessary(ctx, flow); err != nil { return err }
		} else {
			fmt.Println("Already authorized!")
		}

		return nil
	})
}
