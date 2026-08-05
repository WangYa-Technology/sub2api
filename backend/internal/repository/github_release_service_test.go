package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type githubReleaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (f githubReleaseRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestGitHubReleaseClient() *githubReleaseClient {
	return &githubReleaseClient{httpClient: &http.Client{}}
}

func TestGitHubReleaseClientAPIRequestAuthorization(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantAuth string
	}{
		{name: "exact HTTPS authority", url: "https://api.github.com/repos/test/repo", wantAuth: "Bearer update-secret"},
		{name: "HTTP", url: "http://api.github.com/repos/test/repo"},
		{name: "subdomain", url: "https://sub.api.github.com/repos/test/repo"},
		{name: "userinfo", url: "https://user@api.github.com/repos/test/repo"},
		{name: "explicit default port", url: "https://api.github.com:443/repos/test/repo"},
		{name: "custom port", url: "https://api.github.com:8443/repos/test/repo"},
		{name: "different host", url: "https://github.com/test/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestGitHubReleaseClient()
			client.updateGitHubToken = "update-secret"
			req, err := client.newAPIRequest(context.Background(), tt.url)
			require.NoError(t, err)
			require.Equal(t, tt.wantAuth, req.Header.Get("Authorization"))
		})
	}
}

func TestGitHubReleaseClientRedirectAuthorization(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantAuth string
	}{
		{name: "same HTTPS authority", url: "https://api.github.com/redirected", wantAuth: "Bearer update-secret"},
		{name: "HTTP", url: "http://api.github.com/redirected"},
		{name: "subdomain", url: "https://sub.api.github.com/redirected"},
		{name: "userinfo", url: "https://user@api.github.com/redirected"},
		{name: "custom port", url: "https://api.github.com:8443/redirected"},
		{name: "different host", url: "https://example.com/redirected"},
	}

	checkRedirect := githubAPICheckRedirect(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer update-secret")
			require.NoError(t, checkRedirect(req, nil))
			require.Equal(t, tt.wantAuth, req.Header.Get("Authorization"))
		})
	}
}

func TestGitHubReleaseClientFetchLatestRelease(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `{"tag_name":"v1.0.0","name":"Release 1.0.0","body":"Release notes","html_url":"https://github.com/test/repo/releases/v1.0.0"}`,
		},
		{name: "non-200", statusCode: http.StatusNotFound, wantErr: true},
		{name: "invalid JSON", statusCode: http.StatusOK, body: "not valid json", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/repos/test/repo/releases/latest", r.URL.Path)
				require.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
				require.Equal(t, "Sub2API-Updater", r.Header.Get("User-Agent"))
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			client := newTestGitHubReleaseClient()
			client.httpClient.Transport = githubReleaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				clone := req.Clone(req.Context())
				clone.URL.Scheme = "http"
				clone.URL.Host = server.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(clone)
			})

			release, err := client.FetchLatestRelease(context.Background(), "test/repo")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "v1.0.0", release.TagName)
			require.Equal(t, "Release 1.0.0", release.Name)
		})
	}
}

func TestGitHubReleaseClientFetchLatestReleaseContextCancel(t *testing.T) {
	client := newTestGitHubReleaseClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.FetchLatestRelease(ctx, "test/repo")
	require.Error(t, err)
}

func TestGitHubReleaseClientFetchRecentReleases(t *testing.T) {
	tests := []struct {
		name        string
		perPage     int
		wantPerPage string
	}{
		{name: "default", perPage: 0, wantPerPage: "10"},
		{name: "requested", perPage: 20, wantPerPage: "20"},
		{name: "clamped", perPage: 101, wantPerPage: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/repos/test/repo/releases", r.URL.Path)
				require.Equal(t, tt.wantPerPage, r.URL.Query().Get("per_page"))
				_, _ = io.WriteString(w, `[{"tag_name":"v1.1.0","draft":false,"prerelease":true}]`)
			}))
			defer server.Close()

			client := newTestGitHubReleaseClient()
			client.httpClient.Transport = githubReleaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				clone := req.Clone(req.Context())
				clone.URL.Scheme = "http"
				clone.URL.Host = server.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(clone)
			})

			releases, err := client.FetchRecentReleases(context.Background(), "test/repo", tt.perPage)
			require.NoError(t, err)
			require.Len(t, releases, 1)
			require.Equal(t, "v1.1.0", releases[0].TagName)
			require.True(t, releases[0].Prerelease)
		})
	}
}
