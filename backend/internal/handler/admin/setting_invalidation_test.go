//go:build unit

package admin

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUpdateSettings_RedisPublishFailureStillReturnsSuccess(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeySiteName: "before"})
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	require.NoError(t, client.Close())
	calls := 0
	h.settingService.StartFrontendInvalidation(client, func() { calls++ })
	defer h.settingService.StopFrontendInvalidation()
	rec := doUpdateSettings(t, h, map[string]any{"site_name": "after"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "after", repo.values[service.SettingKeySiteName])
	require.Equal(t, 1, calls)
}
