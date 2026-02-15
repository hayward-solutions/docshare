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

type S3Client struct {
	client         *minio.Client
	bucket         string
	publicEndpoint string
}

func NewS3Client(cfg config.S3Config) (*S3Client, error) {
	var creds *credentials.Credentials

	if cfg.AccessKey == "" {
		creds = credentials.NewIAM("")
	} else {
		creds = credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	return &S3Client{
		client:         client,
		bucket:         cfg.Bucket,
		publicEndpoint: cfg.PublicEndpoint,
	}, nil
}

func (s *S3Client) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		logger.Error("s3_upload_failed", err, map[string]interface{}{
			"object_name":  objectName,
			"size":         size,
			"content_type": contentType,
			"bucket":       s.bucket,
		})
	} else {
		logger.Info("s3_upload_success", map[string]interface{}{
			"object_name":  objectName,
			"size":         size,
			"content_type": contentType,
			"bucket":       s.bucket,
		})
	}
	return err
}

func (s *S3Client) Download(ctx context.Context, objectName string) (*minio.Object, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		logger.Error("s3_download_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      s.bucket,
		})
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		logger.Error("s3_download_stat_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      s.bucket,
		})
		return nil, err
	}
	logger.Info("s3_download_success", map[string]interface{}{
		"object_name": objectName,
		"bucket":      s.bucket,
	})
	return obj, nil
}

func (s *S3Client) Delete(ctx context.Context, objectName string) error {
	err := s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		logger.Error("s3_delete_failed", err, map[string]interface{}{
			"object_name": objectName,
			"bucket":      s.bucket,
		})
	} else {
		logger.Info("s3_delete_success", map[string]interface{}{
			"object_name": objectName,
			"bucket":      s.bucket,
		})
	}
	return err
}

func (s *S3Client) PresignedGetURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	urlValue, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return urlValue.String(), nil
}

func (s *S3Client) PresignedGetURLWithResponse(ctx context.Context, objectName string, expiry time.Duration, contentType string, contentDisposition string) (string, error) {
	query := make(url.Values)
	if contentType != "" {
		query.Set("response-content-type", contentType)
	}
	if contentDisposition != "" {
		query.Set("response-content-disposition", contentDisposition)
	}

	urlValue, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, expiry, query)
	if err != nil {
		return "", err
	}
	return urlValue.String(), nil
}

func (s *S3Client) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("failed creating bucket %s: %w", s.bucket, err)
	}
	return nil
}
