package storage

import (
	"context"
	"io"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Filename    string
	StoragePath string
	SizeBytes   int64
}

type Adapter interface {
	ListFiles(ctx context.Context, basePath string) ([]FileInfo, error)
	GetFile(ctx context.Context, storagePath string) (io.ReadCloser, int64, error)
	WriteFile(ctx context.Context, storagePath string, r io.Reader, size int64) error
	DeleteFile(ctx context.Context, storagePath string) error
}

var videoExtensions = map[string]struct{}{
	".mp4": {}, ".mov": {}, ".avi": {}, ".mkv": {}, ".mxf": {},
	".m4v": {}, ".wmv": {}, ".flv": {}, ".ts": {}, ".mts": {},
}

func isVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := videoExtensions[ext]
	return ok
}
