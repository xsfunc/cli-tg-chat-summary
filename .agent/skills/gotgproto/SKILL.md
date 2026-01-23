---
name: gotgproto Telegram Client
description: High-level wrapper for gotd/td with simplified API for Telegram userbot/client development
---

# gotgproto Client Best Practices

## Overview

**gotgproto** is a high-level wrapper around `gotd/td` that simplifies Telegram client development. It provides:
- Automatic session/peer storage management
- Built-in dispatcher with handler groups
- Simplified authentication flow
- Helper methods for common operations
- Support for Pyrogram/Telethon/GramJS session strings

**Version:** v1.0.0-beta22+

## CRITICAL: Security and Ban Prevention

Inherits all security rules from gotd/td:
- **NEVER share APP_ID and APP_HASH**
- Send email to recover@telegram.org before first use explaining your use case
- **DO NOT** use QR code login (permanent ban risk)
- **DO NOT** use VoIP numbers
- Implement rate limiting (mandatory for userbots)
- Handle FLOOD_WAIT properly
- Use client passively (receive more than send)

## Installation

```bash
go get github.com/celestix/gotgproto
```

For SQLite session storage:
```bash
go get github.com/glebarez/sqlite  # CGO-free
# or
go get gorm.io/driver/sqlite       # with CGO
```

## Client Initialization

### User Client (Phone Auth)
```go
import (
    "log"
    "os"
    "strconv"
    
    "github.com/celestix/gotgproto"
    "github.com/celestix/gotgproto/sessionMaker"
    "github.com/glebarez/sqlite"
)

func main() {
    appID, _ := strconv.Atoi(os.Getenv("TG_APP_ID"))
    
    client, err := gotgproto.NewClient(
        appID,
        os.Getenv("TG_API_HASH"),
        gotgproto.ClientTypePhone(os.Getenv("TG_PHONE")),
        &gotgproto.ClientOpts{
            Session: sessionMaker.SqlSession(sqlite.Open("session.db")),
            // Use custom auth conversator for code/password prompts
            AuthConversator: gotgproto.BasicConversator(),
        },
    )
    if err != nil {
        log.Fatalln("failed to start client:", err)
    }

    // Client is ready, Self contains user info
    log.Printf("Logged in as @%s (ID: %d)\n", client.Self.Username, client.Self.ID)

    // Block until client stops
    client.Idle()
}
```

## ClientOpts Reference

```go
type ClientOpts struct {
    // Session storage (required for persistence)
    Session sessionMaker.SessionConstructor
    
    // Use in-memory session (no persistence)
    InMemory bool
    
    // Custom auth handler for phone auth
    AuthConversator AuthConversator
    
    // Dispatcher for handling updates
    Dispatcher dispatcher.Dispatcher
    
    // Logger instance (zap)
    Logger *zap.Logger
    
    // Disable automatic auth retry
    NoAutoAuth bool
    
    // Disable receiving updates
    NoUpdates bool
    
    // DC configuration
    DC int           // Default: 2
    DCList dcs.List  // Custom DC list
    
    // Disable copyright message
    DisableCopyright bool
}
```

## Session Storage Types

### SQLite (Recommended for persistence)
```go
// CGO-free SQLite
import "github.com/glebarez/sqlite"
Session: sessionMaker.SqlSession(sqlite.Open("session.db"))

// GORM SQLite (CGO)
import "gorm.io/driver/sqlite"
Session: sessionMaker.SqlSession(sqlite.Open("session.db"))
```

### In-Memory (for testing)
```go
&gotgproto.ClientOpts{
    InMemory: true,
    Session:  sessionMaker.SimpleSession(),
}
```

### JSON File
```go
Session: sessionMaker.JsonFileSession("session.json")
```

### Import from other libraries
```go
// Pyrogram session string
Session: sessionMaker.PyrogramSession("SESSION_STRING")

// Telethon session string  
Session: sessionMaker.TelethonSession("SESSION_STRING")

// GramJS session string
Session: sessionMaker.GramjsSession("SESSION_STRING")

// gotgproto own string session
Session: sessionMaker.GotgprotoSession("SESSION_STRING")
```

### Export String Session
```go
sessionString, err := client.ExportStringSession()
```

## Dispatcher & Handlers

### Setup Dispatcher
The client comes with a built-in dispatcher accessed via `client.Dispatcher`.

### Handler Types for Userbot

```go
import (
    "github.com/celestix/gotgproto/dispatcher"
    "github.com/celestix/gotgproto/dispatcher/handlers"
    "github.com/celestix/gotgproto/dispatcher/handlers/filters"
    "github.com/celestix/gotgproto/ext"
)

dp := client.Dispatcher

// Message handler with filter
dp.AddHandler(handlers.NewMessage(filters.Message.Text, textHandler))

// Any update handler (useful for logging/monitoring)
dp.AddHandler(handlers.NewAnyUpdate(anyHandler))

// Chat member updates
dp.AddHandler(handlers.NewChatMemberUpdated(filters.ChatMemberUpdated.All, memberHandler))
```

### Handler Groups
Use groups to control handler execution order:

```go
// Group 0 (default) - executed first
dp.AddHandler(handlers.NewMessage(filters.Message.All, firstHandler))

// Group 1 - executed after group 0
dp.AddHandlerToGroup(handlers.NewMessage(filters.Message.All, secondHandler), 1)

// Stop processing further groups
return dispatcher.EndGroups
```

### Handler Signature
```go
func handler(ctx *ext.Context, update *ext.Update) error {
    // ctx - contains client, self, sender helper, peer storage
    // update - contains effective message, user, chat
    return nil
}
```

## Message Filters

```go
// Built-in filters
filters.Message.All      // All messages
filters.Message.Text     // Text messages only
filters.Message.Media    // Media messages
filters.Message.Photo    // Photo messages
filters.Message.Video    // Video messages
filters.Message.Document // Document messages
filters.Message.Audio    // Audio messages
filters.Message.Voice    // Voice messages
filters.Message.Sticker  // Sticker messages

// Chat type filters
filters.Group      // Group chats
filters.Supergroup // Supergroups
filters.Channel    // Channels
```

## ext.Context API

### Sending Messages
```go
// Send to specific chat by ID
ctx.SendMessage(chatID, &tg.MessagesSendMessageRequest{
    Message: "Hello!",
})

// Reply to message in current update
ctx.Reply(update, ext.ReplyTextString("Response"), nil)

// With formatting
ctx.Reply(update, ext.ReplyTextString("<b>Bold</b>"), &ext.ReplyOpts{
    ParseMode: ext.HTML,
})
```

### Sending Media
```go
import "github.com/gotd/td/telegram/uploader"

// Upload file
u := uploader.NewUploader(ctx.Raw)
f, err := u.FromPath(ctx, "file.jpg")

// Send as photo
ctx.SendMedia(chatID, &tg.MessagesSendMediaRequest{
    Message: "Caption",
    Media: &tg.InputMediaUploadedPhoto{
        File: f,
    },
})

// Send as document
ctx.SendMedia(chatID, &tg.MessagesSendMediaRequest{
    Message: "Caption",
    Media: &tg.InputMediaUploadedDocument{
        File:     f,
        MimeType: "video/mp4",
        Attributes: []tg.DocumentAttributeClass{
            &tg.DocumentAttributeFilename{FileName: "video.mp4"},
        },
    },
})
```

### Download Media
```go
import "github.com/celestix/gotgproto/functions"

// Get filename from media
filename, err := functions.GetMediaFileNameWithId(msg.Media)

// Download to file
_, err = ctx.DownloadMedia(
    msg.Media,
    ext.DownloadOutputPath(filename),
    nil,
)

// Download to buffer
buf := &bytes.Buffer{}
_, err = ctx.DownloadMedia(
    msg.Media,
    ext.DownloadOutputBuffer(buf),
    nil,
)
```

### Chat & User Operations
```go
// Get chat info
chat, err := ctx.GetChat(chatID)

// Get user info  
user, err := ctx.GetUser(userID)

// Get messages by IDs
messages, err := ctx.GetMessages(chatID, []tg.InputMessageClass{
    &tg.InputMessageID{ID: msgID},
})

// Edit message
ctx.EditMessage(chatID, &tg.MessagesEditMessageRequest{
    ID:      msgID,
    Message: "Edited text",
})

// Delete messages
ctx.DeleteMessages(chatID, []int{msgID1, msgID2})

// Forward messages
ctx.ForwardMessages(fromChatID, toChatID, &tg.MessagesForwardMessagesRequest{
    ID: []int{msgID},
})

// Archive chats
ctx.ArchiveChats([]int64{chatID1, chatID2})

// Add members to group
ctx.AddChatMembers(chatID, []int64{userID1, userID2}, 100)

// Create group
chat, err := ctx.CreateChat("Group Name", []int64{userID1})

// Create channel
channel, err := ctx.CreateChannel("Channel Name", "Description", false) // false = group, true = channel
```

## ext.Update Structure

```go
type Update struct {
    // Raw update from Telegram
    UpdateClass tg.UpdateClass
    
    // Parsed message (if applicable)
    EffectiveMessage *types.Message
    
    // Chat member update (if applicable)
    ChatMemberUpdated *tg.UpdateChannelParticipant
}

// Helper methods
update.EffectiveUser()    // Get user who triggered update
update.EffectiveChat()    // Get chat where update happened
update.EffectiveMessage   // Access message directly
```

## Working with Raw API (Important for Userbots)

Access full gotd/td API through `ctx.Raw`:

### Get Dialogs (Chats List)
```go
dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
    Limit:      100,
    OffsetPeer: &tg.InputPeerEmpty{},
})

// Type switch to access dialogs
switch d := dialogs.(type) {
case *tg.MessagesDialogs:
    for _, dialog := range d.Dialogs {
        // process dialogs
    }
case *tg.MessagesDialogsSlice:
    for _, dialog := range d.Dialogs {
        // process dialogs
    }
}
```

### Get Chat History
```go
history, err := ctx.Raw.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
    Peer:  ctx.PeerStorage.GetInputPeerById(chatID),
    Limit: 100,
})

switch h := history.(type) {
case *tg.MessagesMessages:
    for _, msg := range h.Messages {
        if m, ok := msg.(*tg.Message); ok {
            log.Println(m.Message)
        }
    }
case *tg.MessagesMessagesSlice:
    for _, msg := range h.Messages {
        if m, ok := msg.(*tg.Message); ok {
            log.Println(m.Message)
        }
    }
case *tg.MessagesChannelMessages:
    for _, msg := range h.Messages {
        if m, ok := msg.(*tg.Message); ok {
            log.Println(m.Message)
        }
    }
}
```

### Search Messages
```go
results, err := ctx.Raw.MessagesSearch(ctx, &tg.MessagesSearchRequest{
    Peer:   ctx.PeerStorage.GetInputPeerById(chatID),
    Q:      "search query",
    Filter: &tg.InputMessagesFilterEmpty{},
    Limit:  50,
})
```

### Get Unread Dialogs
```go
dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
    Limit:      100,
    OffsetPeer: &tg.InputPeerEmpty{},
})

// Filter unread
switch d := dialogs.(type) {
case *tg.MessagesDialogsSlice:
    for _, dialog := range d.Dialogs {
        if dlg, ok := dialog.(*tg.Dialog); ok {
            if dlg.UnreadCount > 0 {
                // This dialog has unread messages
            }
        }
    }
}
```

### Mark Messages as Read
```go
// For regular chats
ctx.Raw.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
    Peer:  ctx.PeerStorage.GetInputPeerById(chatID),
    MaxID: lastMessageID,
})

// For channels
ctx.Raw.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
    Channel: &tg.InputChannel{
        ChannelID:  channelID,
        AccessHash: accessHash,
    },
    MaxID: lastMessageID,
})
```

## Peer Storage

gotgproto automatically manages peer storage. Access it via `ctx.PeerStorage`:

```go
// Get InputPeer by chat ID (works for users, groups, channels)
inputPeer := ctx.PeerStorage.GetInputPeerById(chatID)

// Get InputUser by user ID
inputUser := ctx.PeerStorage.GetInputUserById(userID)
```

## Creating Context Outside Handlers

For operations outside update handlers (e.g., scheduled tasks):

```go
// Create context from client
ctx := client.CreateContext()

// Use context for API calls
dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
    Limit:      100,
    OffsetPeer: &tg.InputPeerEmpty{},
})

// Refresh context if needed (updates entities)
client.RefreshContext(ctx)
```

## Common Mistakes

### 1. Not checking EffectiveMessage for nil
```go
// WRONG - may panic
text := update.EffectiveMessage.Text

// CORRECT
if msg := update.EffectiveMessage; msg != nil {
    text := msg.Text
}
```

### 2. Forgetting dispatcher.EndGroups
```go
// WRONG - continues to all handlers
func myHandler(ctx *ext.Context, update *ext.Update) error {
    // process...
    return nil
}

// CORRECT - stops after this handler
func myHandler(ctx *ext.Context, update *ext.Update) error {
    // process...
    return dispatcher.EndGroups
}
```

### 3. Not handling errors
```go
// WRONG
ctx.SendMessage(chatID, &tg.MessagesSendMessageRequest{Message: "Hi"})

// CORRECT
if _, err := ctx.SendMessage(chatID, &tg.MessagesSendMessageRequest{Message: "Hi"}); err != nil {
    log.Printf("Failed to send: %v", err)
}
```

### 4. Missing Idle() call
```go
// WRONG - exits immediately
client, _ := gotgproto.NewClient(...)
log.Println("Started!")
// program exits

// CORRECT - blocks until client stops
client, _ := gotgproto.NewClient(...)
log.Println("Started!")
client.Idle()
```

### 5. Type assertions without check
```go
// WRONG - panic if not *tg.Message
m := msg.(*tg.Message)

// CORRECT
m, ok := msg.(*tg.Message)
if !ok {
    return nil
}
```

### 6. Processing own messages (infinite loop)
```go
// WRONG
func handler(ctx *ext.Context, u *ext.Update) error {
    msg := u.EffectiveMessage
    ctx.Reply(u, ext.ReplyTextString(msg.Text), nil) // echoes own messages!
    return nil
}

// CORRECT - skip outgoing messages
func handler(ctx *ext.Context, u *ext.Update) error {
    msg := u.EffectiveMessage
    if msg == nil || msg.Out {
        return nil  // skip outgoing
    }
    // process only incoming
    return nil
}
```

## Complete Example: Chat Summary Client

```go
package main

import (
    "log"
    "os"
    "strconv"

    "github.com/celestix/gotgproto"
    "github.com/celestix/gotgproto/dispatcher/handlers"
    "github.com/celestix/gotgproto/dispatcher/handlers/filters"
    "github.com/celestix/gotgproto/ext"
    "github.com/celestix/gotgproto/sessionMaker"
    "github.com/glebarez/sqlite"
    "github.com/gotd/td/tg"
)

func main() {
    appID, _ := strconv.Atoi(os.Getenv("TG_APP_ID"))
    
    client, err := gotgproto.NewClient(
        appID,
        os.Getenv("TG_API_HASH"),
        gotgproto.ClientTypePhone(os.Getenv("TG_PHONE")),
        &gotgproto.ClientOpts{
            Session:         sessionMaker.SqlSession(sqlite.Open("session.db")),
            AuthConversator: gotgproto.BasicConversator(),
        },
    )
    if err != nil {
        log.Fatalln("failed to start:", err)
    }

    log.Printf("Logged in as @%s\n", client.Self.Username)

    // Get dialogs with unread messages
    ctx := client.CreateContext()
    dialogs, err := ctx.Raw.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
        Limit:      100,
        OffsetPeer: &tg.InputPeerEmpty{},
    })
    if err != nil {
        log.Fatalln("failed to get dialogs:", err)
    }

    processDialogs(ctx, dialogs)

    // Set up handler for new messages
    client.Dispatcher.AddHandler(handlers.NewMessage(
        filters.Message.All,
        func(ctx *ext.Context, u *ext.Update) error {
            msg := u.EffectiveMessage
            if msg == nil || msg.Out {
                return nil
            }
            log.Printf("New message from %d: %s\n", msg.PeerID, msg.Text)
            return nil
        },
    ))

    client.Idle()
}

func processDialogs(ctx *ext.Context, dialogs tg.MessagesDialogsClass) {
    switch d := dialogs.(type) {
    case *tg.MessagesDialogsSlice:
        for _, dialog := range d.Dialogs {
            if dlg, ok := dialog.(*tg.Dialog); ok {
                if dlg.UnreadCount > 0 {
                    log.Printf("Chat has %d unread messages\n", dlg.UnreadCount)
                }
            }
        }
    case *tg.MessagesDialogs:
        for _, dialog := range d.Dialogs {
            if dlg, ok := dialog.(*tg.Dialog); ok {
                if dlg.UnreadCount > 0 {
                    log.Printf("Chat has %d unread messages\n", dlg.UnreadCount)
                }
            }
        }
    }
}
```

## Comparison: gotgproto vs gotd/td

| Feature | gotd/td | gotgproto |
|---------|---------|-----------|
| Session storage | Manual setup | Built-in (SQLite, JSON, memory) |
| Peer storage | Manual with contrib | Automatic |
| Auth flow | Manual implementation | Simplified with AuthConversator |
| Dispatching | Basic UpdateDispatcher | Advanced with filters/groups |
| Helper methods | Limited | Rich (SendMessage, GetChat, etc.) |
| String sessions | Not supported | Pyrogram/Telethon/GramJS import |
| Learning curve | Steeper | Easier |

## When to Use Which

**Use gotgproto when:**
- Building userbots/clients quickly
- Need built-in session/peer management
- Want simplified update handling
- Migrating from Pyrogram/Telethon
- Need convenience methods for common operations

**Use raw gotd/td when:**
- Need maximum control and performance
- Building complex MTProto applications
- Custom update handling required
- Already familiar with MTProto

## Links

- Repository: https://github.com/celestix/gotgproto
- Documentation: https://pkg.go.dev/github.com/celestix/gotgproto
- Examples: https://github.com/celestix/gotgproto/tree/beta/examples
- gotd/td (underlying library): https://github.com/gotd/td
- API Credentials: https://my.telegram.org/apps
