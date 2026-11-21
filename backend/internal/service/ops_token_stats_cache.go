package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// OpsTokenStatsCacheKey includes every result dimension and the current global
// administrator scope. Timezone is not a dimension of this rolling-window report.
// v2 isolates duration-weighted TPS from the v1 arithmetic mean during rolling
// upgrades and rollback. The repository's global compute lock stays shared.
func OpsTokenStatsCacheKey(f *OpsOpenAITokenStatsFilter) string {
	group := int64(0)
	if f.GroupID != nil {
		group = *f.GroupID
	}
	raw := fmt.Sprintf("admin-global:v2:%s:%s:%s:%d:%s:%d:%d:%d", f.StartTime.UTC().Format(time.RFC3339Nano), f.EndTime.UTC().Format(time.RFC3339Nano), strings.ToLower(strings.TrimSpace(f.Platform)), group, f.TimeRange, f.Page, f.PageSize, f.TopN)
	sum := sha256.Sum256([]byte(raw))
	return "ops:token-stats:v2:" + hex.EncodeToString(sum[:])
}

func (s *OpsService) getCachedTokenStats(ctx context.Context, filter *OpsOpenAITokenStatsFilter) (*OpsOpenAITokenStatsResponse, error) {
	f := *filter
	result := s.tokenStatsFlight.DoChan(OpsTokenStatsCacheKey(&f), func() (any, error) {
		workCtx, cancel := context.WithTimeout(ctx, 58*time.Second)
		defer cancel()
		return s.opsRepo.GetOpenAITokenStats(workCtx, &f)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-result:
		if out.Err != nil {
			return nil, out.Err
		}
		return out.Val.(*OpsOpenAITokenStatsResponse), nil
	}
}
