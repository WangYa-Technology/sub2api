package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAsyncImageOpsTestContext(path string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Set(asyncImageOriginalPathContextKey, path)
	c.Set(ctxKeyInboundEndpoint, EndpointImagesGenerationsAsync)
	setOpsRequestContext(c, "gpt-image-1", false)
	setOpsEndpointContext(c, "gpt-image-1", int16(service.RequestTypeAsync))
	groupID := int64(9)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      7,
		Key:     "sk-async-test-key",
		GroupID: &groupID,
		User:    &service.User{ID: 5},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
	})
	return c
}

func TestRecordAsyncImageOpsOutcomeRecordsTerminalStorageFailure(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	c := newAsyncImageOpsTestContext("/v1/images/generations/async")

	recordAsyncImageOpsOutcome(c, ops, http.StatusBadGateway, []byte(`{"error":{"type":"api_error","message":"failed to store generated image to object storage"}}`))

	job := <-opsErrorLogQueue
	require.NotNil(t, job.entry)
	require.Equal(t, http.StatusBadGateway, job.entry.StatusCode)
	require.Equal(t, EndpointImagesGenerationsAsync, job.entry.InboundEndpoint)
	require.Equal(t, EndpointImagesGenerations, job.entry.UpstreamEndpoint)
	require.Equal(t, "/v1/images/generations/async", job.entry.RequestPath)
	require.NotNil(t, job.entry.RequestType)
	require.Equal(t, int16(service.RequestTypeAsync), *job.entry.RequestType)
	require.False(t, job.entry.Stream)
	require.Contains(t, job.entry.ErrorMessage, "object storage")
	require.Empty(t, opsErrorLogQueue, "one terminal failure must produce exactly one error record")
}

func TestRecordAsyncImageOpsOutcomeAggregatesRecoveredUpstreamErrors(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	c := newAsyncImageOpsTestContext("/v1/images/generations/async")
	c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
		AccountID:          12,
		UpstreamStatusCode: http.StatusBadGateway,
		Message:            "first account failed",
	}})

	recordAsyncImageOpsOutcome(c, ops, http.StatusOK, []byte(`{"data":[]}`))

	job := <-opsErrorLogQueue
	require.NotNil(t, job.entry)
	require.Equal(t, http.StatusOK, job.entry.StatusCode)
	require.Equal(t, "upstream_error", job.entry.ErrorType)
	require.NotNil(t, job.entry.UpstreamErrorsJSON)
	require.Contains(t, *job.entry.UpstreamErrorsJSON, "first account failed")
	require.Contains(t, job.entry.ErrorMessage, "Recovered upstream error 502")
	require.Equal(t, EndpointImagesGenerationsAsync, job.entry.InboundEndpoint)
	require.NotNil(t, job.entry.RequestType)
	require.Equal(t, int16(service.RequestTypeAsync), *job.entry.RequestType)
	require.Empty(t, opsErrorLogQueue, "one recovered attempt set must produce exactly one error record")
}
