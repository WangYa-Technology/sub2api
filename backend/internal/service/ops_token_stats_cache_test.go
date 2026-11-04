package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type tokenStatsCountingRepo struct {
	OpsRepository
	calls atomic.Int32
	query func(context.Context, *OpsOpenAITokenStatsFilter) (*OpsOpenAITokenStatsResponse, error)
}

func (r *tokenStatsCountingRepo) GetOpenAITokenStats(ctx context.Context, f *OpsOpenAITokenStatsFilter) (*OpsOpenAITokenStatsResponse, error) {
	r.calls.Add(1)
	return r.query(ctx, f)
}
func tokenStatsTestFilter() *OpsOpenAITokenStatsFilter {
	end := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	return &OpsOpenAITokenStatsFilter{TimeRange: "30d", StartTime: end.Add(-30 * 24 * time.Hour), EndTime: end, TopN: 20}
}
func TestTokenStatsCacheKeyIsolation(t *testing.T) {
	f := tokenStatsTestFilter()
	base := OpsTokenStatsCacheKey(f)
	changes := []func(*OpsOpenAITokenStatsFilter){
		func(f *OpsOpenAITokenStatsFilter) { f.Platform = "openai" },
		func(f *OpsOpenAITokenStatsFilter) { id := int64(3); f.GroupID = &id },
		func(f *OpsOpenAITokenStatsFilter) { f.TopN = 50 },
		func(f *OpsOpenAITokenStatsFilter) { f.TopN = 0; f.Page = 2; f.PageSize = 20 },
		func(f *OpsOpenAITokenStatsFilter) { f.StartTime = f.StartTime.Add(time.Minute) },
		func(f *OpsOpenAITokenStatsFilter) { f.EndTime = f.EndTime.Add(time.Minute) },
		func(f *OpsOpenAITokenStatsFilter) { f.TimeRange = "15d" },
	}
	for _, change := range changes {
		clone := *f
		change(&clone)
		require.NotEqual(t, base, OpsTokenStatsCacheKey(&clone))
	}
	f.Platform = " OpenAI "
	clone := *f
	clone.Platform = "openai"
	require.Equal(t, OpsTokenStatsCacheKey(f), OpsTokenStatsCacheKey(&clone))
}
func TestTokenStatsSingleflightAndWaiterCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	repo := &tokenStatsCountingRepo{query: func(ctx context.Context, _ *OpsOpenAITokenStatsFilter) (*OpsOpenAITokenStatsResponse, error) {
		close(started)
		select {
		case <-release:
			return &OpsOpenAITokenStatsResponse{Total: 42}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	svc := &OpsService{opsRepo: repo}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		out, err := svc.getCachedTokenStats(context.Background(), tokenStatsTestFilter())
		if err != nil || out.Total != 42 {
			t.Errorf("owner result: %v %v", out, err)
		}
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := svc.getCachedTokenStats(ctx, tokenStatsTestFilter())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), repo.calls.Load())
	close(release)
	wg.Wait()
}
func TestTokenStatsOwnerCancellationReachesRepository(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	repo := &tokenStatsCountingRepo{query: func(ctx context.Context, _ *OpsOpenAITokenStatsFilter) (*OpsOpenAITokenStatsResponse, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}}
	svc := &OpsService{opsRepo: repo}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := svc.getCachedTokenStats(ctx, tokenStatsTestFilter()); done <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("repository did not receive cancellation")
	}
}
