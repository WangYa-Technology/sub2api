package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestNewS3ImageStorageSelectsConfiguredClient(t *testing.T) {
	base := config.ImageStorageConfig{
		Endpoint:        "https://storage.example.com",
		Region:          "us-east-1",
		Bucket:          "images",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		ForcePathStyle:  true,
	}

	awsStorage, err := NewS3ImageStorage(context.Background(), &base)
	require.NoError(t, err)
	require.NotNil(t, awsStorage.client)
	require.Nil(t, awsStorage.minioClient)

	base.UseMinIOClient = true
	minioStorage, err := NewS3ImageStorage(context.Background(), &base)
	require.NoError(t, err)
	require.Nil(t, minioStorage.client)
	require.NotNil(t, minioStorage.minioClient)
}

func TestNewMinIOImageClientRejectsEndpointPath(t *testing.T) {
	_, err := newMinIOImageClient(&config.ImageStorageConfig{
		Endpoint:        "https://storage.example.com/api",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
	})
	require.ErrorContains(t, err, "must not contain a path")
}
