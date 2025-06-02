// Binary bot-auth-manual implements example of custom session storage and
// manually setting up client options without environment variables.
package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"sync"

	"github.com/golang/glog"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
)

const (
	telegramAppID    = "TELEGRAM_APP_ID"
	telegramAppHash  = "TELEGRAM_APP_HASH"
	telegramBotToken = "TELEGRAM_BOT_TOKEN"
)

// memorySession implements in-memory session storage.
// Goroutine-safe.
type memorySession struct {
	mux  sync.RWMutex
	data []byte
}

// LoadSession loads session from memory.
func (s *memorySession) LoadSession(context.Context) ([]byte, error) {
	if s == nil {
		return nil, session.ErrNotFound
	}

	s.mux.RLock()
	defer s.mux.RUnlock()

	if len(s.data) == 0 {
		return nil, session.ErrNotFound
	}

	cpy := append([]byte(nil), s.data...)

	return cpy, nil
}

// StoreSession stores session to memory.
func (s *memorySession) StoreSession(ctx context.Context, data []byte) error {
	s.mux.Lock()
	s.data = data
	s.mux.Unlock()
	return nil
}

func main() {
	// Grab those from https://my.telegram.org/apps.
	flag.Parse()
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	var missingVars []string
	appID, err := strconv.Atoi(os.Getenv(telegramAppID))
	if err != nil {
		missingVars = append(missingVars, telegramAppID)
	}

	appHash := os.Getenv(telegramAppHash)
	if appHash == "" {
		missingVars = append(missingVars, telegramAppHash)
	}

	botToken := os.Getenv(telegramBotToken)
	if botToken == "" {
		missingVars = append(missingVars, telegramBotToken)
	}

	if missingVars != nil {
		glog.Fatalf("Missing the following ENV variables: %v", missingVars)
	}

	// Using custom session storage.
	// You can save session to database, e.g. Redis, MongoDB or postgres.
	// See memorySession for implementation details.
	sessionStorage := &memorySession{}
	ctx := context.Background()

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: sessionStorage,
	})

	if err := client.Run(ctx, func(ctx context.Context) error {
		// Checking auth status.
		status, err := client.Auth().Status(ctx)
		if err != nil {

			return err
		}
		// Can be already authenticated if we have valid session in
		// session storage.
		if !status.Authorized {
			// Otherwise, perform bot authentication.
			if _, err := client.Auth().Bot(ctx, botToken); err != nil {
				return err
			}
		}

		// All good, manually authenticated.
		glog.Info("Done")

		return nil
	}); err != nil {
    glog.Fatalf("Failed to auth with %s", err)
  }

}
