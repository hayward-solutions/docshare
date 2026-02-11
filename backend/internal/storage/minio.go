package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/docshare/backend/internal/config"
	"github.com/docshare/backend/pkg/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	client            *minio.Client
	publicClient      *minio.Client // Separate client for presigned URLs with public endpoint
	bucket            string
	publicEndpoint    string
	usePublicEndpoint bool
}

func NewMinIOClient(cfg config.MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinIOClient{
		client:         client,
		bucket:         cfg.Bucket,
		publicEndpoint: cfg.PublicEndpoint,
	}, nil
}

func (m *MinIOClient) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		logger.Error("minio_upload_failed", err, map[string]interface{}{
			"object_name":  objectName,
			"size":         size,
			"content_type": contentType,
			"bucket":       m.bucket,
		})
	} else {
		logger.Info("minio_upload_success", map[string]interface{}{
			"object_name":  objectName,
			"size":         size,
			"content_type": contentType,
			"bucket":       m.bucket,
		})
	}
	return err
}

func (m *MinIOClient) Download(ctx context.Context, objectName string) (*minio.Object, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		logger.Error("minio_download_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      m.bucket,
		})
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		logger.Error("minio_download_stat_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      m.bucket,
		})
		return nil, err
	}
	logger.Info("minio_download_success", map[string]interface{}{
		"object_name": objectName,
		"bucket":      m.bucket,
	})
	return obj, nil
}

func (m *MinIOClient) Delete(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		logger.Error("minio_delete_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      m.bucket,
		})
	} else {
		logger.Info("minio_delete_success", map[string]interface{}{
			"object_name": objectName,
			"bucket":      m.bucket,
		})
	}
	return err
}

func (m *MinIOClient) PresignedGetURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	client := m.client
	if m.usePublicEndpoint && m.publicClient != nil {
		client = m.publicClient
	}
	urlValue, err := client.PresignedGetObject(ctx, m.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return urlValue.String(), nil
}

func (m *MinIOClient) PresignedGetURLWithResponse(ctx context.Context, objectName string, expiry time.Duration, contentType string, contentDisposition string) (string, error) {
	query := make(url.Values)
	if contentType != "" {
		query.Set("response-content-type", contentType)
	}
	if contentDisposition != "" {
		query.Set("response-content-disposition", contentDisposition)
	}

	urlValue, err := m.client.PresignedGetObject(ctx, m.bucket, objectName, expiry, query)
	if err != nil {
		return "", err
	}
	return urlValue.String(), nil
}

func (m *MinIOClient) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("failed creating bucket %s: %w", m.bucket, err)
	}
	return nil
}
