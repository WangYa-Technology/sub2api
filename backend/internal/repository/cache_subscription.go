package repository

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// subscribeCacheUpdates owns only a PubSub connection, never the shared client.
// Confirmations (including reconnect) trigger reconciliation because Redis does
// not replay notifications sent while a subscriber was disconnected.
func subscribeCacheUpdates(parent context.Context, client *redis.Client, channel string, handler func()) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if client == nil {
			return
		}
		sub := client.Subscribe(ctx, channel)
		closed := make(chan struct{})
		go func() { defer close(closed); <-ctx.Done(); _ = sub.Close() }()
		defer func() { cancel(); <-closed }()
		for ctx.Err() == nil {
			msg, err := sub.Receive(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("cache subscription unavailable; retrying", "channel", channel, "error", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
				continue
			}
			switch msg.(type) {
			case *redis.Subscription, *redis.Message:
				handler()
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}
