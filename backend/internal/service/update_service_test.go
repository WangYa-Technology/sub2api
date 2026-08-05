//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateCheckCacheStub struct {
	data string
}

func (s *updateCheckCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateCheckCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateCheckGitHubStub struct {
	release *GitHubRelease
}

func (s *updateCheckGitHubStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateCheckGitHubStub) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
	return nil, nil
}

func TestCompareVersionsUsesNumericBaseVersion(t *testing.T) {
	tests := []struct {
		name            string
		current, latest string
		want            int
	}{
		{name: "custom suffix equals base release", current: "v0.1.164-fix", latest: "v0.1.164", want: 0},
		{name: "hcai suffix equals base release", current: "v0.1.165-hcai", latest: "v0.1.165", want: 0},
		{name: "build metadata equals base release", current: "v0.1.164+hcai", latest: "v0.1.164", want: 0},
		{name: "base release equals custom suffix", current: "v0.1.164", latest: "v0.1.164-fix", want: 0},
		{name: "newer custom patch", current: "v0.1.165-fix", latest: "v0.1.164", want: 1},
		{name: "older custom patch", current: "v0.1.163-fix", latest: "v0.1.164", want: -1},
		{name: "normal newer release", current: "0.1.164", latest: "0.1.165", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, compareVersions(tt.current, tt.latest))
		})
	}
}

func TestCheckUpdateDoesNotTreatBaseReleaseAsNewerThanCustomizedBuild(t *testing.T) {
	cache := &updateCheckCacheStub{}
	svc := NewUpdateService(cache, &updateCheckGitHubStub{release: &GitHubRelease{
		TagName: "v0.1.164",
		Name:    "v0.1.164",
	}}, "v0.1.164-fix", "release")

	fresh, err := svc.CheckUpdate(context.Background(), true)
	require.NoError(t, err)
	require.False(t, fresh.HasUpdate)
	require.False(t, fresh.Cached)

	cached, err := svc.CheckUpdate(context.Background(), false)
	require.NoError(t, err)
	require.False(t, cached.HasUpdate)
	require.True(t, cached.Cached)
}
