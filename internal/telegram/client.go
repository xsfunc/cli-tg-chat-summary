package telegram

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"cli-tg-chat-summary/internal/config"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/td/tg"
)

type Client struct {
	cfg       *config.Config
	proto     *gotgproto.Client
	ctx       *ext.Context
	peerCache map[int64]tg.InputPeerClass
}

type Chat struct {
	ID          int64
	Title       string
	UnreadCount int
	IsChannel   bool
	LastReadID  int
}

type Message struct {
	ID     int
	Date   time.Time
	Text   string
	Sender string
}

func NewClient(cfg *config.Config) (*Client, error) {
	return &Client{
		cfg:       cfg,
		peerCache: make(map[int64]tg.InputPeerClass),
	}, nil
}

func (c *Client) Login(input io.Reader) error {
	// Ensure session directory exists
	if err := os.MkdirAll("session", 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	client, err := gotgproto.NewClient(
		c.cfg.TelegramAppID,
		c.cfg.TelegramAppHash,
		gotgproto.ClientTypePhone(c.cfg.Phone),
		&gotgproto.ClientOpts{
			Session:         sessionMaker.SqlSession(sqlite.Open("session/session.db")),
			AuthConversator: gotgproto.BasicConversator(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	c.proto = client
	c.ctx = client.CreateContext()

	return nil
}

func (c *Client) GetDialogs() ([]Chat, error) {
	if c.ctx == nil {
		c.ctx = c.proto.CreateContext()
	}

	dialogs, err := c.ctx.Raw.MessagesGetDialogs(c.ctx, &tg.MessagesGetDialogsRequest{
		Limit:      100,
		OffsetPeer: &tg.InputPeerEmpty{},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get dialogs: %w", err)
	}

	var parsedDialogs []Chat

	switch d := dialogs.(type) {
	case *tg.MessagesDialogsSlice:
		parsedDialogs = c.processDialogs(d.Dialogs, d.Chats, d.Users)
	case *tg.MessagesDialogs:
		parsedDialogs = c.processDialogs(d.Dialogs, d.Chats, d.Users)
	}

	// Sort by unread count desc
	sort.Slice(parsedDialogs, func(i, j int) bool {
		return parsedDialogs[i].UnreadCount > parsedDialogs[j].UnreadCount
	})

	return parsedDialogs, nil
}

func (c *Client) processDialogs(dialogs []tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass) []Chat {
	chatMap := make(map[int64]tg.ChatClass)
	for _, ch := range chats {
		chatMap[ch.GetID()] = ch
		switch item := ch.(type) {
		case *tg.Chat:
			c.peerCache[item.ID] = &tg.InputPeerChat{ChatID: item.ID}
		case *tg.Channel:
			c.peerCache[item.ID] = &tg.InputPeerChannel{ChannelID: item.ID, AccessHash: item.AccessHash}
		}
	}
	userMap := make(map[int64]tg.UserClass)
	for _, u := range users {
		userMap[u.GetID()] = u
		switch item := u.(type) {
		case *tg.User:
			c.peerCache[item.ID] = &tg.InputPeerUser{UserID: item.ID, AccessHash: item.AccessHash}
		}
	}

	var results []Chat

	for _, d := range dialogs {
		dlg, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}

		var title string
		var peerID int64
		var isChannel bool

		switch p := dlg.Peer.(type) {
		case *tg.PeerUser:
			peerID = p.UserID
			if u, ok := userMap[peerID]; ok {
				switch user := u.(type) {
				case *tg.User:
					title = user.FirstName + " " + user.LastName
					if user.Username != "" {
						title += " (@" + user.Username + ")"
					}
				}
			}
		case *tg.PeerChat:
			peerID = p.ChatID
			if ch, ok := chatMap[peerID]; ok {
				switch chat := ch.(type) {
				case *tg.Chat:
					title = chat.Title
				}
			}
		case *tg.PeerChannel:
			peerID = p.ChannelID
			isChannel = true
			if ch, ok := chatMap[peerID]; ok {
				switch channel := ch.(type) {
				case *tg.Channel:
					title = channel.Title
				}
			}
		}

		if title == "" {
			title = fmt.Sprintf("Unknown Peer %d", peerID)
		}

		results = append(results, Chat{
			ID:          peerID,
			Title:       title,
			UnreadCount: dlg.UnreadCount,
			IsChannel:   isChannel,
			LastReadID:  dlg.ReadInboxMaxID,
		})
	}
	return results
}

func (c *Client) GetUnreadMessages(chatID int64, lastReadID int) ([]Message, error) {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		// Fallback to storage if not in cache (though unlikely for dialogs)
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}

	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found in cache or storage", chatID)
	}

	var allMessages []Message
	offsetID := 0
	batchSize := 100

	// Check loop: fetch until we find messages <= lastReadID OR end of history
	for {
		// Rate limit: wait 1 second between requests
		time.Sleep(1 * time.Second)

		req := &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			Limit:    batchSize,
			OffsetID: offsetID,
		}

		history, err := c.ctx.Raw.MessagesGetHistory(c.ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get history: %w", err)
		}

		var msgs []tg.MessageClass
		var users []tg.UserClass

		switch h := history.(type) {
		case *tg.MessagesMessages:
			msgs = h.Messages
			users = h.Users
		case *tg.MessagesMessagesSlice:
			msgs = h.Messages
			users = h.Users
		case *tg.MessagesChannelMessages:
			msgs = h.Messages
			users = h.Users
		}

		if len(msgs) == 0 {
			break
		}

		// Map users for quick lookup
		userMap := make(map[int64]tg.UserClass)
		for _, u := range users {
			userMap[u.GetID()] = u
		}

		lastID := 0
		foundRead := false

		for _, m := range msgs {
			msg, ok := m.(*tg.Message)
			if !ok {
				continue
			}
			lastID = msg.ID

			// Stop if we reached the last read message
			if msg.ID <= lastReadID {
				foundRead = true
				break // Stop processing this batch
			}

			if msg.Message == "" {
				continue
			}

			if msg.Out {
				continue
			}

			sender := "Unknown"
			if msg.FromID != nil {
				switch p := msg.FromID.(type) {
				case *tg.PeerUser:
					// Try local batch map first
					if u, ok := userMap[p.UserID]; ok {
						switch user := u.(type) {
						case *tg.User:
							sender = user.FirstName + " " + user.LastName
							if sender == " " {
								sender = "Deleted Account"
							}
						}
					} else {
						// Fallback to general resolution
						name, err := c.getSenderName(p.UserID)
						if err == nil {
							sender = name
						}
					}
				}
			}
			if sender == " " || sender == "" {
				sender = "Unknown"
			}

			allMessages = append(allMessages, Message{
				ID:     msg.ID,
				Date:   time.Unix(int64(msg.Date), 0),
				Text:   msg.Message,
				Sender: sender,
			})
		}

		if foundRead {
			break
		}

		offsetID = lastID

		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

func (c *Client) MarkAsRead(chat Chat, maxID int) error {
	inputPeer, ok := c.peerCache[chat.ID]
	if !ok {
		// Fallback to storage
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chat.ID)
	}

	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chat.ID)
	}

	if chat.IsChannel {
		inputChannel, ok := inputPeer.(*tg.InputPeerChannel)
		if !ok {
			// Try to cast or reconstruct if possible, but peerCache should have correct type
			return fmt.Errorf("peer is marked as channel but input peer is %T", inputPeer)
		}

		// For channels/supergroups
		_, err := c.ctx.Raw.ChannelsReadHistory(c.ctx, &tg.ChannelsReadHistoryRequest{
			Channel: &tg.InputChannel{
				ChannelID:  inputChannel.ChannelID,
				AccessHash: inputChannel.AccessHash,
			},
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark channel as read: %w", err)
		}
	} else {
		// For users and basic groups
		_, err := c.ctx.Raw.MessagesReadHistory(c.ctx, &tg.MessagesReadHistoryRequest{
			Peer:  inputPeer,
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark chat as read: %w", err)
		}
	}

	return nil
}

func (c *Client) getSenderName(userID int64) (string, error) {
	// Try cache first
	if inputPeer, ok := c.peerCache[userID]; ok {
		// Check against self
		if c.proto.Self.ID == userID {
			return c.proto.Self.FirstName, nil
		}

		if userPeer, ok := inputPeer.(*tg.InputPeerUser); ok {
			inputUser := &tg.InputUser{
				UserID:     userPeer.UserID,
				AccessHash: userPeer.AccessHash,
			}
			return c.fetchUserName(inputUser)
		}
	}

	// Fallback to storage
	inputPeer := c.ctx.PeerStorage.GetInputPeerById(userID)
	if inputPeer == nil {
		return "", fmt.Errorf("user not found in storage")
	}

	switch p := inputPeer.(type) {
	case *tg.InputPeerUser:
		return c.fetchUserName(&tg.InputUser{UserID: p.UserID, AccessHash: p.AccessHash})
	case *tg.InputPeerSelf:
		if c.proto.Self.ID == userID {
			return c.proto.Self.FirstName, nil
		}
		return "Me", nil
	}

	return "", fmt.Errorf("peer is not a user")
}

func (c *Client) fetchUserName(inputUser tg.InputUserClass) (string, error) {
	res, err := c.ctx.Raw.UsersGetUsers(c.ctx, []tg.InputUserClass{inputUser})
	if err != nil {
		return "", err
	}
	if len(res) > 0 {
		switch user := res[0].(type) {
		case *tg.User:
			return user.FirstName, nil
		}
	}
	return "", fmt.Errorf("user not found")
}
