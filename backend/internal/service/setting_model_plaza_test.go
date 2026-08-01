package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type modelPlazaSettingRepoStub struct {
	SettingRepository
	values map[string]string
	err    error
}

func (s *modelPlazaSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func TestParseModelPlazaCNYPerUSD(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		raw  string
		want float64
	}{
		{name: "configured", raw: "0.25", want: 0.25},
		{name: "missing", raw: "", want: ModelPlazaCNYPerUSDDefault},
		{name: "zero", raw: "0", want: ModelPlazaCNYPerUSDDefault},
		{name: "negative", raw: "-1", want: ModelPlazaCNYPerUSDDefault},
		{name: "nan", raw: "NaN", want: ModelPlazaCNYPerUSDDefault},
		{name: "infinity", raw: "+Inf", want: ModelPlazaCNYPerUSDDefault},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, parseModelPlazaCNYPerUSD(test.raw))
		})
	}
}

func TestGetModelPlazaRuntimeIncludesCNYPerUSD(t *testing.T) {
	t.Parallel()

	repo := &modelPlazaSettingRepoStub{values: map[string]string{
		SettingKeyModelPlazaEnabled:     "true",
		SettingKeyModelPlazaRequireAuth: "true",
		SettingKeyModelPlazaDescription: "pricing notes",
		SettingKeyModelPlazaCNYPerUSD:   "0.25",
	}}
	svc := NewSettingService(repo, &config.Config{})

	runtime := svc.GetModelPlazaRuntime(context.Background())
	require.True(t, runtime.Enabled)
	require.True(t, runtime.RequireAuth)
	require.Equal(t, "pricing notes", runtime.Description)
	require.Equal(t, 0.25, runtime.CNYPerUSD)
}

func TestGetModelPlazaRuntimeFailsClosed(t *testing.T) {
	t.Parallel()

	svc := NewSettingService(
		&modelPlazaSettingRepoStub{err: errors.New("database unavailable")},
		&config.Config{},
	)

	runtime := svc.GetModelPlazaRuntime(context.Background())
	require.False(t, runtime.Enabled)
}
