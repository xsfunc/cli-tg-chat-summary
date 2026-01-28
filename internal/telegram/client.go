package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/gotd/td/bin"

	"cli-tg-chat-summary/internal/config"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"
)

type Client struct {
	cfg          *config.Config
	proto        *gotgproto.Client
	ctx          *ext.Context
	peerCache    map[int64]tg.InputPeerClass
	channelCache map[int64]*tg.Channel // For forum operations
}

type Chat struct {
	ID           int64
	Title        string
	UnreadCount  int
	IsChannel    bool
	IsForum      bool
	LastReadID   int
	TopMessageID int
}

type Topic struct {
	ID          int
	Title       string
	UnreadCount int
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
		cfg:          cfg,
		peerCache:    make(map[int64]tg.InputPeerClass),
		channelCache: make(map[int64]*tg.Channel),
	}, nil
}

func (c *Client) Login(ctx context.Context, input io.Reader) error {
	// Configure logger
	var level slog.Level
	switch c.cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	// Ensure session directory exists
	if err := os.MkdirAll("session", 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	opts := &gotgproto.ClientOpts{
		Session:         sessionMaker.SqlSession(sqlite.Open("session/session.db")),
		AuthConversator: gotgproto.BasicConversator(),
		Middlewares: []telegram.Middleware{
			floodwait.NewSimpleWaiter(),
			ratelimit.New(rate.Every(time.Duration(c.cfg.RateLimitMs)*time.Millisecond), 3),
		},
	}

	if c.cfg.LogLevel == "debug" {
		opts.Middlewares = append(opts.Middlewares, MiddlewareFunc(func(next tg.Invoker) telegram.InvokeFunc {
			return telegram.InvokeFunc(func(ctx context.Context, input bin.Encoder, output bin.Decoder) error {
				slog.Debug("TG Request", "method", fmt.Sprintf("%T", input))
				return next.Invoke(ctx, input, output)
			})
		}))
	}

	client, err := gotgproto.NewClient(
		c.cfg.TelegramAppID,
		c.cfg.TelegramAppHash,
		gotgproto.ClientTypePhone(c.cfg.Phone),
		opts,
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	c.proto = client
	c.ctx = client.CreateContext()

	return nil
}

func (c *Client) GetDialogs(ctx context.Context) ([]Chat, error) {
	if c.ctx == nil {
		c.ctx = c.proto.CreateContext()
	}

	dialogs, err := c.ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
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
			c.channelCache[item.ID] = item // Cache for forum operations
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
		var isForum bool

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
					isForum = channel.Forum
				}
			}
		}

		if title == "" {
			title = fmt.Sprintf("Unknown Peer %d", peerID)
		}

		results = append(results, Chat{
			ID:           peerID,
			Title:        title,
			UnreadCount:  dlg.UnreadCount,
			IsChannel:    isChannel,
			IsForum:      isForum,
			LastReadID:   dlg.ReadInboxMaxID,
			TopMessageID: dlg.TopMessage,
		})
	}
	return results
}

func (c *Client) GetUnreadMessages(ctx context.Context, chatID int64, lastReadID int) ([]Message, error) {
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
		// Rate limit is handled by middleware
		req := &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			Limit:    batchSize,
			OffsetID: offsetID,
		}

		history, err := c.ctx.Raw.MessagesGetHistory(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get history: %w", err)
		}

		msgs, users := extractMessagesAndUsers(history)
		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := c.processMessageBatch(ctx, msgs, users, func(msg *tg.Message) (bool, bool) {
			if msg.ID <= lastReadID {
				return false, true // Stop
			}
			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		})

		allMessages = append(allMessages, batchMessages...)

		if stop {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

func (c *Client) MarkAsRead(ctx context.Context, chat Chat, maxID int) error {
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
		_, err := c.ctx.Raw.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
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
		_, err := c.ctx.Raw.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
			Peer:  inputPeer,
			MaxID: maxID,
		})
		if err != nil {
			return fmt.Errorf("failed to mark chat as read: %w", err)
		}
	}

	return nil
}

func (c *Client) getSenderName(ctx context.Context, userID int64) (string, error) {
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
			return c.fetchUserName(ctx, inputUser)
		}
	}

	// Fallback to storage
	inputPeer := c.ctx.PeerStorage.GetInputPeerById(userID)
	if inputPeer == nil {
		return "", fmt.Errorf("user not found in storage")
	}

	switch p := inputPeer.(type) {
	case *tg.InputPeerUser:
		return c.fetchUserName(ctx, &tg.InputUser{UserID: p.UserID, AccessHash: p.AccessHash})
	case *tg.InputPeerSelf:
		if c.proto.Self.ID == userID {
			return c.proto.Self.FirstName, nil
		}
		return "Me", nil
	}

	return "", fmt.Errorf("peer is not a user")
}

func (c *Client) fetchUserName(ctx context.Context, inputUser tg.InputUserClass) (string, error) {
	res, err := c.ctx.Raw.UsersGetUsers(ctx, []tg.InputUserClass{inputUser})
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

// GetForumTopics fetches all topics from a forum with their unread counts.
func (c *Client) GetForumTopics(ctx context.Context, chatID int64) ([]Topic, error) {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	topics, err := c.ctx.Raw.MessagesGetForumTopics(ctx, &tg.MessagesGetForumTopicsRequest{
		Peer:  inputPeer,
		Limit: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get forum topics: %w", err)
	}

	var result []Topic
	for _, t := range topics.Topics {
		topic, ok := t.(*tg.ForumTopic)
		if !ok {
			continue
		}
		result = append(result, Topic{
			ID:          topic.ID,
			Title:       topic.Title,
			UnreadCount: topic.UnreadCount,
			LastReadID:  topic.ReadInboxMaxID,
		})
	}

	return result, nil
}

// GetTopicMessages fetches unread messages from a specific topic.
func (c *Client) GetTopicMessages(ctx context.Context, chatID int64, topicID int, lastReadID int) ([]Message, error) {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	var allMessages []Message
	offsetID := 0
	batchSize := 100

	for {
		// Rate limit is handled by middleware
		var msgs []tg.MessageClass
		var users []tg.UserClass

		if topicID == 1 {
			// General topic - use regular GetHistory but filter messages without ReplyTo.TopMsgID
			req := &tg.MessagesGetHistoryRequest{
				Peer:     inputPeer,
				Limit:    batchSize,
				OffsetID: offsetID,
			}
			history, err := c.ctx.Raw.MessagesGetHistory(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to get history: %w", err)
			}
			msgs, users = extractMessagesAndUsers(history)
		} else {
			// Non-general topic - use GetReplies
			req := &tg.MessagesGetRepliesRequest{
				Peer:     inputPeer,
				MsgID:    topicID,
				Limit:    batchSize,
				OffsetID: offsetID,
			}
			replies, err := c.ctx.Raw.MessagesGetReplies(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to get topic replies: %w", err)
			}
			msgs, users = extractMessagesAndUsers(replies)
		}

		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := c.processMessageBatch(ctx, msgs, users, func(msg *tg.Message) (bool, bool) {
			// For General topic, filter to only include messages without ReplyTo or with top_msg_id=0
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false // Skip message belonging to another topic
					}
				}
			}

			if msg.ID <= lastReadID {
				return false, true // Stop
			}

			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		})

		allMessages = append(allMessages, batchMessages...)

		if stop {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

// MarkTopicAsRead marks a specific topic as read up to the given message ID.
func (c *Client) MarkTopicAsRead(ctx context.Context, chatID int64, topicID int, maxID int) error {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}
	if inputPeer == nil {
		return fmt.Errorf("peer %d not found", chatID)
	}

	_, err := c.ctx.Raw.MessagesReadDiscussion(ctx, &tg.MessagesReadDiscussionRequest{
		Peer:      inputPeer,
		MsgID:     topicID,
		ReadMaxID: maxID,
	})
	if err != nil {
		return fmt.Errorf("failed to mark topic as read: %w", err)
	}

	return nil
}

// resolveMissingUsers identifies users in the message batch that are missing from the provided userMap
// and fetches them in a single batch request to avoid N+1 rate limit issues.
func (c *Client) resolveMissingUsers(ctx context.Context, msgs []tg.MessageClass, userMap map[int64]tg.UserClass) {
	var usersToFetch []tg.InputUserClass
	usersToFetchIDs := make(map[int64]bool)

	for _, m := range msgs {
		msg, ok := m.(*tg.Message)
		if !ok || msg.Out || msg.FromID == nil {
			continue
		}

		var userID int64
		if p, ok := msg.FromID.(*tg.PeerUser); ok {
			userID = p.UserID
		} else {
			continue
		}

		// If user is not in map, queue for fetching
		if _, found := userMap[userID]; !found {
			if !usersToFetchIDs[userID] {
				usersToFetchIDs[userID] = true

				// Try to construct InputUser from cache or storage
				var inputUser tg.InputUserClass

				// 1. Try Cache
				if ip, ok := c.peerCache[userID]; ok {
					if uPeer, ok := ip.(*tg.InputPeerUser); ok {
						inputUser = &tg.InputUser{UserID: uPeer.UserID, AccessHash: uPeer.AccessHash}
					}
				}

				// 2. Try Storage if cache failed
				if inputUser == nil {
					ip := c.ctx.PeerStorage.GetInputPeerById(userID)
					if ip != nil {
						if uPeer, ok := ip.(*tg.InputPeerUser); ok {
							inputUser = &tg.InputUser{UserID: uPeer.UserID, AccessHash: uPeer.AccessHash}
						}
					}
				}

				if inputUser != nil {
					usersToFetch = append(usersToFetch, inputUser)
				}
			}
		}
	}

	// Fetch missing users in one go
	if len(usersToFetch) > 0 {
		// Use a short timeout context if possible, but for now use existing ctx
		res, err := c.ctx.Raw.UsersGetUsers(ctx, usersToFetch)
		if err == nil {
			for _, u := range res {
				userMap[u.GetID()] = u
			}
		}
	}
}

// GetMessagesByDate fetches messages within a specific date range.
func (c *Client) GetMessagesByDate(ctx context.Context, chatID int64, since, until time.Time) ([]Message, error) {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	var allMessages []Message
	offsetID := 0
	batchSize := 100

	for {
		req := &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			Limit:    batchSize,
			OffsetID: offsetID,
		}

		history, err := c.ctx.Raw.MessagesGetHistory(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get history: %w", err)
		}

		msgs, users := extractMessagesAndUsers(history)
		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := c.processMessageBatch(ctx, msgs, users, func(msg *tg.Message) (bool, bool) {
			msgTime := time.Unix(int64(msg.Date), 0)

			// Messages are newest first
			if msgTime.Before(since) {
				return false, true // Stop (tooOld)
			}

			if msgTime.After(until) {
				return false, false // Skip (tooNew)
			}

			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		})

		allMessages = append(allMessages, batchMessages...)

		if stop {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

// GetTopicMessagesByDate fetches topic messages within a specific date range.
func (c *Client) GetTopicMessagesByDate(ctx context.Context, chatID int64, topicID int, since, until time.Time) ([]Message, error) {
	inputPeer, ok := c.peerCache[chatID]
	if !ok {
		inputPeer = c.ctx.PeerStorage.GetInputPeerById(chatID)
	}
	if inputPeer == nil {
		return nil, fmt.Errorf("peer %d not found", chatID)
	}

	var allMessages []Message
	offsetID := 0
	batchSize := 100

	for {
		var msgs []tg.MessageClass
		var users []tg.UserClass

		if topicID == 1 {
			req := &tg.MessagesGetHistoryRequest{
				Peer:     inputPeer,
				Limit:    batchSize,
				OffsetID: offsetID,
			}
			history, err := c.ctx.Raw.MessagesGetHistory(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to get history: %w", err)
			}
			msgs, users = extractMessagesAndUsers(history)
		} else {
			req := &tg.MessagesGetRepliesRequest{
				Peer:     inputPeer,
				MsgID:    topicID,
				Limit:    batchSize,
				OffsetID: offsetID,
			}
			replies, err := c.ctx.Raw.MessagesGetReplies(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to get topic replies: %w", err)
			}
			msgs, users = extractMessagesAndUsers(replies)
		}

		if len(msgs) == 0 {
			break
		}

		batchMessages, lastID, stop := c.processMessageBatch(ctx, msgs, users, func(msg *tg.Message) (bool, bool) {
			if topicID == 1 {
				if msg.ReplyTo != nil {
					if reply, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && reply.ReplyToTopID != 0 {
						return false, false
					}
				}
			}

			msgTime := time.Unix(int64(msg.Date), 0)

			if msgTime.Before(since) {
				return false, true // Stop (tooOld)
			}

			if msgTime.After(until) {
				return false, false // Skip (tooNew)
			}

			if msg.Message == "" || msg.Out {
				return false, false // Skip
			}
			return true, false // Process
		})

		allMessages = append(allMessages, batchMessages...)

		if stop {
			break
		}

		offsetID = lastID
		if len(msgs) < batchSize {
			break
		}
	}

	return allMessages, nil
}

func (c *Client) resolveSender(ctx context.Context, fromID tg.PeerClass, userMap map[int64]tg.UserClass) string {
	sender := "Unknown"
	if fromID != nil {
		switch p := fromID.(type) {
		case *tg.PeerUser:
			if u, ok := userMap[p.UserID]; ok {
				switch user := u.(type) {
				case *tg.User:
					sender = user.FirstName + " " + user.LastName
					if sender == " " {
						sender = "Deleted Account"
					}
				}
			} else {
				name, err := c.getSenderName(ctx, p.UserID)
				if err == nil {
					sender = name
				}
			}
		}
	}
	if sender == " " || sender == "" {
		sender = "Unknown"
	}
	return sender
}

func extractMessagesAndUsers(result tg.MessagesMessagesClass) ([]tg.MessageClass, []tg.UserClass) {
	switch r := result.(type) {
	case *tg.MessagesMessages:
		return r.Messages, r.Users
	case *tg.MessagesMessagesSlice:
		return r.Messages, r.Users
	case *tg.MessagesChannelMessages:
		return r.Messages, r.Users
	}
	return nil, nil
}

func (c *Client) processMessageBatch(ctx context.Context, msgs []tg.MessageClass, users []tg.UserClass,
	filter func(msg *tg.Message) (process bool, stop bool)) ([]Message, int, bool) {

	userMap := make(map[int64]tg.UserClass)
	for _, u := range users {
		userMap[u.GetID()] = u
	}

	c.resolveMissingUsers(ctx, msgs, userMap)

	var results []Message
	var lastID int
	var stopLoop bool

	for _, m := range msgs {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}

		lastID = msg.ID

		process, stop := filter(msg)
		if stop {
			stopLoop = true
			break
		}
		if !process {
			continue
		}

		sender := c.resolveSender(ctx, msg.FromID, userMap)
		results = append(results, Message{
			ID:     msg.ID,
			Date:   time.Unix(int64(msg.Date), 0),
			Text:   msg.Message,
			Sender: sender,
		})
	}
	return results, lastID, stopLoop
}

// Helpers for middleware

// Helpers for middleware

type MiddlewareFunc func(next tg.Invoker) telegram.InvokeFunc

func (m MiddlewareFunc) Handle(next tg.Invoker) telegram.InvokeFunc {
	return m(next)
}
