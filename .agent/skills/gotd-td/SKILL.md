---
name: gotd/td Telegram Client
description: Best practices and patterns for gotd/td MTProto Telegram client library
---

# gotd/td Client Best Practices

## CRITICAL: Security and Ban Prevention

**NEVER share APP_ID and APP_HASH** - they cannot be regenerated and are bound to your account.

All unofficial clients are monitored by Telegram. Accounts using unofficial clients are automatically under observation.

**Before first use:**
- Send email to recover@telegram.org with phone number
- Explain what your client will do
- Ask not to ban your account

**DO NOT:**
- Use QR code login (permanent ban)
- Use VoIP numbers
- Spam, flood, or abuse API
- Share credentials

**DO:**
- Implement rate limiting (mandatory)
- Use client passively (receive more than send)
- Handle FLOOD_WAIT properly

**If banned:** Email recover@telegram.org explaining your use case.

**Support:**
- @gotd_en (English)
- @gotd_ru (Russian)
- @gotd_news (news)

## Client Initialization

```go
import (
    "github.com/gotd/td/telegram"
    "go.uber.org/zap"
)

// From environment variables
client, err := telegram.ClientFromEnvironment(telegram.Options{
    Logger:        log,
    UpdateHandler: updateHandler,
})

// Manual setup
client := telegram.NewClient(appID, appHash, telegram.Options{
    SessionStorage: sessionStorage,
    Logger:         log,
    UpdateHandler:  dispatcher,
    Middlewares:    middlewares,
})
```

## Environment Variables

```bash
APP_ID="12345"
APP_HASH="your_hash"
SESSION_FILE="./session.json"
SESSION_DIR="./session"  # if SESSION_FILE not set
TG_PHONE="+1234567890"   # optional
```

## Session Storage

File storage:
```go
sessionStorage := &telegram.FileSessionStorage{
    Path: filepath.Join(sessionDir, "session.json"),
}
```

Custom storage (for database):
```go
type customSession struct {
    mux  sync.RWMutex
    data []byte
}

func (s *customSession) LoadSession(context.Context) ([]byte, error) {
    if s == nil || len(s.data) == 0 {
        return nil, session.ErrNotFound  // MUST return this if not found
    }
    s.mux.RLock()
    defer s.mux.RUnlock()
    cpy := append([]byte(nil), s.data...)  // MUST return copy
    return cpy, nil
}

func (s *customSession) StoreSession(ctx context.Context, data []byte) error {
    s.mux.Lock()
    s.data = data
    s.mux.Unlock()
    return nil
}
```

## Authentication

```go
import "github.com/gotd/td/telegram/auth"

// Terminal authenticator
type termAuth struct{ phone string }

func (a termAuth) Phone(context.Context) (string, error) {
    if a.phone != "" {
        return a.phone, nil
    }
    // Prompt user
    return phone, nil
}

func (termAuth) Code(_ context.Context, *tg.AuthSentCode) (string, error) {
    // Prompt for code
    return code, nil
}

func (termAuth) Password(context.Context) (string, error) {
    // Prompt for 2FA password
    return password, nil
}

func (termAuth) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
    return &auth.SignUpRequired{TermsOfService: tos}
}

func (termAuth) SignUp(context.Context) (auth.UserInfo, error) {
    return auth.UserInfo{}, errors.New("not implemented")
}

// Usage
flow := auth.NewFlow(termAuth{phone: os.Getenv("TG_PHONE")}, auth.SendCodeOptions{})
if err := client.Auth().IfNecessary(ctx, flow); err != nil {
    return err
}
```

## Updates Handling

Basic dispatcher:
```go
dispatcher := tg.NewUpdateDispatcher()

dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
    m, ok := u.Message.(*tg.Message)
    if !ok || m.Out {  // ALWAYS check type and skip outgoing
        return nil
    }
    // Handle message
    return nil
})

opts := telegram.Options{
    UpdateHandler: dispatcher,
}
```

With gap recovery:
```go
import (
    "github.com/gotd/td/telegram/updates"
    boltstor "github.com/gotd/contrib/bbolt"
    "go.etcd.io/bbolt"
)

boltdb, _ := bbolt.Open("updates.bolt.db", 0666, nil)
gaps := updates.New(updates.Config{
    Handler: dispatcher,
    Logger:  log.Named("gaps"),
    Storage: boltstor.NewStateStorage(boltdb),
})

client, _ := telegram.ClientFromEnvironment(telegram.Options{
    UpdateHandler: gaps,
})

// In Run
gaps.Run(ctx, client.API(), userID, updates.AuthOptions{})
```

## Peer Storage

```go
import (
    pebbledb "github.com/cockroachdb/pebble"
    "github.com/gotd/contrib/pebble"
    "github.com/gotd/contrib/storage"
)

db, _ := pebbledb.Open("peers.pebble.db", &pebbledb.Options{})
peerDB := pebble.NewPeerStorage(db)

dispatcher := tg.NewUpdateDispatcher()
updateHandler := storage.UpdateHook(dispatcher, peerDB)

opts := telegram.Options{
    UpdateHandler: updateHandler,
}
```

## Sending Messages

```go
import (
    "github.com/gotd/td/telegram/message"
    "github.com/gotd/td/telegram/message/html"
)

api := tg.NewClient(client)
sender := message.NewSender(api)

// Text
sender.To(peer).Text(ctx, "Hello")

// HTML
sender.To(peer).StyledText(ctx, html.String(nil, "<b>Bold</b>"))

// Reply
sender.Reply(entities, update).Text(ctx, "Reply")

// Resolve username
sender.Resolve("@username").Text(ctx, "Message")
```

## File Upload

```go
import (
    "github.com/gotd/td/telegram/uploader"
    "github.com/gotd/td/telegram/message"
)

u := uploader.NewUploader(api)
sender := message.NewSender(api).WithUploader(u)

// From path
upload, _ := u.FromPath(ctx, "/path/to/file")

// From bytes
upload, _ := u.FromBytes(ctx, "file.txt", data)

// Send document
doc := message.UploadedDocument(upload)
sender.To(peer).Media(ctx, doc)

// Send photo
photo := message.UploadedPhoto(upload)
sender.To(peer).Media(ctx, photo)
```

## File Download

```go
import "github.com/gotd/td/telegram/downloader"

d := downloader.NewDownloader()

// To file
d.Download(api, location).ToPath(ctx, "/path")

// To memory
buf := &bytes.Buffer{}
d.Download(api, location).Stream(ctx, buf)
```

## Middleware

FLOOD_WAIT handling (mandatory):
```go
import "github.com/gotd/contrib/middleware/floodwait"

waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
    log.Warn("Flood wait", zap.Duration("wait", wait.Duration))
})

opts := telegram.Options{
    Middlewares: []telegram.Middleware{waiter},
}
```

Rate limiting (mandatory for userbot):
```go
import (
    "github.com/gotd/contrib/middleware/ratelimit"
    "golang.org/x/time/rate"
)

opts := telegram.Options{
    Middlewares: []telegram.Middleware{
        ratelimit.New(rate.Every(100*time.Millisecond), 5),
    },
}
```

## Graceful Shutdown

```go
import (
    "os/signal"
    "syscall"
)

ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

client.Run(ctx, func(ctx context.Context) error {
    // Your code
    <-ctx.Done()
    return nil
})
```

## Common Mistakes

1. Type assertion without check:
```go
// WRONG
m := u.Message.(*tg.Message)  // panic if not *tg.Message

// CORRECT
m, ok := u.Message.(*tg.Message)
if !ok {
    return nil
}
```

2. Processing outgoing messages:
```go
// WRONG - creates infinite loop
dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
    m, ok := u.Message.(*tg.Message)
    if !ok {
        return nil
    }
    sender.Reply(e, u).Text(ctx, "Echo")  // replies to own messages!
    return nil
})

// CORRECT
if !ok || m.Out {
    return nil  // skip outgoing
}
```

3. Session storage not returning ErrNotFound:
```go
// WRONG
func (s *customSession) LoadSession(ctx context.Context) ([]byte, error) {
    if notFound {
        return nil, errors.New("not found")  // wrong error
    }
    return data, nil
}

// CORRECT
if notFound {
    return nil, session.ErrNotFound  // specific error
}
```

4. Not copying data in LoadSession:
```go
// WRONG - caller can modify internal data
return s.data, nil

// CORRECT - return copy
return append([]byte(nil), s.data...), nil
```

5. No FLOOD_WAIT handling:
```go
// WRONG - will fail on FLOOD_WAIT
opts := telegram.Options{Logger: log}

// CORRECT
opts := telegram.Options{
    Logger: log,
    Middlewares: []telegram.Middleware{
        floodwait.NewWaiter(),
    },
}
```

6. No graceful shutdown:
```go
// WRONG
ctx := context.Background()
client.Run(ctx, ...)

// CORRECT
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
defer cancel()
client.Run(ctx, ...)
```

7. Forgetting log.Sync():
```go
// WRONG
log, _ := zap.NewDevelopment()
client.Run(ctx, ...)

// CORRECT
log, _ := zap.NewDevelopment()
defer func() { _ = log.Sync() }()
client.Run(ctx, ...)
```

## Complete Example

```go
package main

import (
    "context"
    "os"
    "os/signal"
    
    "github.com/gotd/td/telegram"
    "github.com/gotd/td/telegram/auth"
    "github.com/gotd/td/telegram/updates"
    "github.com/gotd/td/tg"
    "github.com/gotd/contrib/middleware/floodwait"
    "go.uber.org/zap"
)

func main() {
    log, _ := zap.NewDevelopment()
    defer func() { _ = log.Sync() }()
    
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    
    dispatcher := tg.NewUpdateDispatcher()
    
    client, _ := telegram.ClientFromEnvironment(telegram.Options{
        Logger:        log,
        UpdateHandler: dispatcher,
        Middlewares: []telegram.Middleware{
            floodwait.NewWaiter(),
        },
    })
    
    dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
        m, ok := u.Message.(*tg.Message)
        if !ok || m.Out {
            return nil
        }
        log.Info("Message", zap.String("text", m.Message))
        return nil
    })
    
    client.Run(ctx, func(ctx context.Context) error {
        flow := auth.NewFlow(termAuth{}, auth.SendCodeOptions{})
        if err := client.Auth().IfNecessary(ctx, flow); err != nil {
            return err
        }
        
        user, _ := client.Self(ctx)
        log.Info("Logged in", zap.String("username", user.Username))
        
        <-ctx.Done()
        return nil
    })
}
```

## Project Structure

```
myapp/
├── cmd/myapp/main.go
├── internal/
│   ├── config/config.go
│   ├── telegram/
│   │   ├── client.go
│   │   ├── auth.go
│   │   └── handlers.go
│   └── storage/
│       ├── session.go
│       └── peers.go
├── session/
└── go.mod
```

## Links

- Documentation: https://pkg.go.dev/github.com/gotd/td
- Examples: https://github.com/gotd/td/tree/main/examples
- API credentials: https://my.telegram.org/apps
