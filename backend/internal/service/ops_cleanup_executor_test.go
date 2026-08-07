package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// deliveryKeyCleanupQuery 必须与 deleteOldWeComDeliveryKeys 中的 SQL 逐字节一致
// （QueryMatcherEqual 做精确匹配）。修改函数内 SQL 时需同步此常量。
const deliveryKeyCleanupQuery = `
WITH batch AS (
  SELECT id FROM settings
  WHERE key LIKE $1
    AND updated_at < $2
  ORDER BY id
  LIMIT $3
)
DELETE FROM settings
WHERE id IN (SELECT id FROM batch)`

func TestDeleteOldWeComDeliveryKeys(t *testing.T) {
	prefix := opsWeComDeliveryKeyPrefix + "%"

	t.Run("nil db no-ops", func(t *testing.T) {
		n, err := deleteOldWeComDeliveryKeys(context.Background(), nil, 30, 100)
		require.NoError(t, err)
		require.Zero(t, n)
	})

	t.Run("negative retention skips without touching db", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, -1, 100)
		require.NoError(t, err)
		require.Zero(t, n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("zero retention prunes all delivery keys with epoch cutoff", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, time.Unix(0, 0).UTC(), 5000).
			WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, time.Unix(0, 0).UTC(), 5000).
			WillReturnResult(sqlmock.NewResult(0, 0))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 0, 5000)
		require.NoError(t, err)
		require.Equal(t, int64(3), n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("positive retention deletes rows older than cutoff", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 5000).
			WillReturnResult(sqlmock.NewResult(0, 5))
		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 5000).
			WillReturnResult(sqlmock.NewResult(0, 0))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 7, 5000)
		require.NoError(t, err)
		require.Equal(t, int64(5), n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("loops until a batch affects zero rows", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 200).
			WillReturnResult(sqlmock.NewResult(0, 200))
		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 200).
			WillReturnResult(sqlmock.NewResult(0, 200))
		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 200).
			WillReturnResult(sqlmock.NewResult(0, 0))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 30, 200)
		require.NoError(t, err)
		require.Equal(t, int64(400), n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("defaults batch size when non-positive", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), opsCleanupBatchSize).
			WillReturnResult(sqlmock.NewResult(0, 0))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 30, 0)
		require.NoError(t, err)
		require.Zero(t, n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing relation treated as no-op", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 5000).
			WillReturnError(errors.New(`pq: relation "settings" does not exist`))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 30, 5000)
		require.NoError(t, err)
		require.Zero(t, n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("other db errors propagate", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(deliveryKeyCleanupQuery).
			WithArgs(prefix, sqlmock.AnyArg(), 5000).
			WillReturnError(errors.New("connection refused"))

		n, err := deleteOldWeComDeliveryKeys(context.Background(), db, 30, 5000)
		require.Error(t, err)
		require.Zero(t, n)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestOpsCleanupDeletedCountsStringIncludesWeComKeys(t *testing.T) {
	c := opsCleanupDeletedCounts{weComDeliveryKeys: 12}
	require.Contains(t, c.String(), "wecom_delivery_keys=12")
}
