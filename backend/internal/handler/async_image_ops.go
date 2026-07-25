package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// recordAsyncImageOpsOutcome finalizes the background half of an asynchronous
// image request. The submit response has already returned 202, so the regular
// middleware cannot observe this terminal response or recovered upstream
// attempts.
func recordAsyncImageOpsOutcome(c *gin.Context, ops *service.OpsService, statusCode int, body []byte) {
	if c == nil || ops == nil || c.Request == nil {
		return
	}
	ctx := context.WithoutCancel(c.Request.Context())
	if !ops.IsMonitoringEnabled(ctx) || shouldSkipOpsErrorLogForCyber(c) {
		return
	}
	if value, ok := c.Get(service.OpsSkipPassthroughKey); ok {
		if skip, _ := value.(bool); skip {
			return
		}
	}

	apiKey := getOpsAPIKey(c)
	platform := resolveOpsPlatform(ctx, apiKey, guessPlatformFromPath(asyncImageRequestPath(c)))
	entry := newAsyncImageOpsEntry(c, apiKey, platform, statusCode)
	applyOpsLatencyFieldsFromContext(c, entry)
	applyOpsUpstreamFieldsFromContext(c, entry)

	if statusCode < 400 {
		if entry.UpstreamStatusCode == nil && entry.UpstreamErrorMessage == nil && entry.UpstreamErrorDetail == nil && len(entry.UpstreamErrors) == 0 {
			return
		}
		entry.ErrorType = "upstream_error"
		entry.ErrorMessage = "Recovered upstream error"
		if entry.UpstreamStatusCode != nil && *entry.UpstreamStatusCode > 0 {
			entry.ErrorMessage += fmt.Sprintf(" %d", *entry.UpstreamStatusCode)
		}
		if entry.UpstreamErrorMessage != nil {
			entry.ErrorMessage += ": " + strings.TrimSpace(*entry.UpstreamErrorMessage)
		}
		entry.ErrorMessage = truncateString(entry.ErrorMessage, 2048)
		classifyStatus := 0
		if entry.UpstreamStatusCode != nil {
			classifyStatus = *entry.UpstreamStatusCode
		}
		entry.ErrorPhase, entry.IsBusinessLimited, entry.ErrorOwner, entry.ErrorSource = classifyOpsErrorLog(
			c,
			entry.ErrorType,
			entry.ErrorMessage,
			"",
			classifyStatus,
		)
		entry.Severity = classifyOpsSeverity(entry.ErrorType, classifyStatus)
		enqueueOpsErrorLog(ops, entry)
		return
	}

	parsed := parseOpsErrorResponse(body)
	if shouldSkipOpsErrorLog(ctx, ops, parsed.Message, string(body), asyncImageRequestPath(c)) {
		return
	}
	entry.ErrorType = normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	entry.ErrorMessage = parsed.Message
	entry.ErrorBody = string(body)
	entry.ErrorPhase, entry.IsBusinessLimited, entry.ErrorOwner, entry.ErrorSource = classifyOpsErrorLog(
		c,
		entry.ErrorType,
		parsed.Message,
		parsed.Code,
		statusCode,
	)
	entry.Severity = classifyOpsSeverity(entry.ErrorType, statusCode)
	enqueueOpsErrorLog(ops, entry)
}

func newAsyncImageOpsEntry(
	c *gin.Context,
	apiKey *service.APIKey,
	platform string,
	statusCode int,
) *service.OpsInsertErrorLogInput {
	requestType := int16(service.RequestTypeAsync)
	clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
	model, _ := c.Get(opsModelKey)
	modelName, _ := model.(string)
	accountIDValue, _ := c.Get(opsAccountIDKey)
	var accountID *int64
	if value, ok := accountIDValue.(int64); ok && value > 0 {
		accountID = &value
	}
	requestID := c.Writer.Header().Get("X-Request-Id")
	if requestID == "" {
		requestID = c.Writer.Header().Get("x-request-id")
	}
	entry := &service.OpsInsertErrorLogInput{
		RequestID:        requestID,
		ClientRequestID:  clientRequestID,
		AccountID:        accountID,
		Platform:         platform,
		Model:            strings.TrimSpace(modelName),
		RequestPath:      asyncImageRequestPath(c),
		Stream:           false,
		InboundEndpoint:  GetInboundEndpoint(c),
		UpstreamEndpoint: GetUpstreamEndpoint(c, platform),
		RequestedModel:   strings.TrimSpace(modelName),
		RequestType:      &requestType,
		UserAgent:        c.GetHeader("User-Agent"),
		StatusCode:       statusCode,
		CreatedAt:        time.Now(),
	}
	if value, ok := c.Get(opsUpstreamModelKey); ok {
		if upstreamModel, ok := value.(string); ok {
			entry.UpstreamModel = strings.TrimSpace(upstreamModel)
		}
	}
	if apiKey != nil {
		entry.APIKeyID = &apiKey.ID
		entry.APIKeyPrefix = keyPrefix(apiKey.Key, 8)
		if apiKey.User != nil {
			entry.UserID = &apiKey.User.ID
		} else if apiKey.UserID > 0 {
			entry.UserID = &apiKey.UserID
		}
		if apiKey.GroupID != nil {
			entry.GroupID = apiKey.GroupID
		}
		if apiKey.Group != nil && apiKey.Group.Platform != "" {
			entry.Platform = apiKey.Group.Platform
		}
	}
	if clientIP := strings.TrimSpace(ip.GetClientIP(c)); clientIP != "" {
		entry.ClientIP = &clientIP
	}
	return entry
}

func asyncImageRequestPath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if value, ok := c.Get(asyncImageOriginalPathContextKey); ok {
		if path, ok := value.(string); ok && strings.TrimSpace(path) != "" {
			return strings.TrimSpace(path)
		}
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}
