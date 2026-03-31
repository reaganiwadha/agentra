package storage

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type MinIOAdapter struct {
	client *minio.Client
	bucket string
}

func newMinIOAdapter(cfg domain.StorageConfig) (*MinIOAdapter, error) {
	endpoint := cfg.Endpoint.String
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	secure := u.Scheme == "https"
	host := u.Host
	if host == "" {
		host = endpoint
	}

	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey.String, cfg.SecretKey.String, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}

	return &MinIOAdapter{client: client, bucket: cfg.Bucket.String}, nil
}

func (a *MinIOAdapter) ListFiles(ctx context.Context, basePath string) ([]FileInfo, error) {
	prefix := strings.TrimPrefix(basePath, "/")
	var files []FileInfo
	for obj := range a.client.ListObjects(ctx, a.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		if isVideoFile(obj.Key) {
			files = append(files, FileInfo{
				Filename:    filepath.Base(obj.Key),
				StoragePath: obj.Key,
				SizeBytes:   obj.Size,
			})
		}
	}
	return files, nil
}

func (a *MinIOAdapter) GetFile(ctx context.Context, storagePath string) (io.ReadCloser, int64, error) {
	obj, err := a.client.GetObject(ctx, a.bucket, storagePath, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, err
	}
	stat, err := obj.Stat()
	if err != nil {
		return nil, 0, err
	}
	return obj, stat.Size, nil
}

func (a *MinIOAdapter) WriteFile(ctx context.Context, storagePath string, r io.Reader, size int64) error {
	exists, err := a.client.BucketExists(ctx, a.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := a.client.MakeBucket(ctx, a.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}

	_, err = a.client.PutObject(ctx, a.bucket, storagePath, r, size, minio.PutObjectOptions{})
	return err
}

func (a *MinIOAdapter) DeleteFile(ctx context.Context, storagePath string) error {
	return a.client.RemoveObject(ctx, a.bucket, storagePath, minio.RemoveObjectOptions{})
}
