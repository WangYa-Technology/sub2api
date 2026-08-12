package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildContentModerationLogWhere_BlockedIncludesAllBlockActions(t *testing.T) {
	where, args := buildContentModerationLogWhere(service.ContentModerationLogFilter{Result: "blocked"})

	require.Empty(t, args)
	sql := strings.Join(where, " AND ")
	require.Contains(t, sql, "l.action IN ('block', 'keyword_block', 'hash_block')")
	require.NotContains(t, sql, "l.action = 'block'")
}

func TestContentModerationCleanupClaimUsesSharedPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	bucket := "2026-08-11"
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO settings (key, value, updated_at)")).
		WithArgs(contentModerationCleanupClaimSettingKey, bucket).
		WillReturnRows(sqlmock.NewRows([]string{"claimed"}).AddRow(true))

	acquired, err := repo.TryClaimContentModerationCleanup(context.Background(), bucket)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationUserViolationTransactionSerializesAutoBan(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	since := time.Now().Add(-time.Hour)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT role, status FROM users WHERE id = $1 FOR UPDATE")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"role", "status"}).AddRow(service.RoleUser, service.StatusActive))
	mock.ExpectQuery("INSERT INTO content_moderation_logs").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(99), time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(int64(42), since, false).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2")).
		WithArgs(service.StatusDisabled, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE content_moderation_logs SET violation_count = $1, auto_banned = $2 WHERE id = $3")).
		WithArgs(3, true, int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	userID := int64(42)
	log := &service.ContentModerationLog{UserID: &userID, Flagged: true, CategoryScores: map[string]float64{}, ThresholdSnapshot: map[string]float64{}}
	applied, admin, err := repo.PersistContentModerationFlaggedLog(context.Background(), log, since, false, true, 3)
	require.NoError(t, err)
	require.Equal(t, 3, log.ViolationCount)
	require.True(t, log.AutoBanned)
	require.True(t, applied)
	require.False(t, admin)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryCountFlaggedByUserSince_ExcludesHashBlock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("AND action <> 'hash_block'")).
		WithArgs(int64(1001), since, false).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountFlaggedByUserSince(context.Background(), 1001, since, false)

	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContentModerationRepositoryCountFlaggedByUserSince_ExcludesCyberPolicyWhenRequested(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewContentModerationRepository(db)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(regexp.QuoteMeta("AND ($3::bool IS FALSE OR action <> 'cyber_policy')")).
		WithArgs(int64(1001), since, true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := repo.CountFlaggedByUserSince(context.Background(), 1001, since, true)

	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.NoError(t, mock.ExpectationsWereMet())
}
