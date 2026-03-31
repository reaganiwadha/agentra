package storage

import (
	"context"
	"io"
	"net"
	"strings"

	"github.com/hirochachacha/go-smb2"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type SMBAdapter struct {
	cfg domain.StorageConfig
}

func newSMBAdapter(cfg domain.StorageConfig) *SMBAdapter {
	return &SMBAdapter{cfg: cfg}
}

func (a *SMBAdapter) connect() (*smb2.Share, func(), error) {
	host := a.cfg.Endpoint.String
	if !strings.Contains(host, ":") {
		host = host + ":445"
	}

	conn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, nil, err
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     a.cfg.AccessKey.String,
			Password: a.cfg.SecretKey.String,
		},
	}
	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	share, err := session.Mount(a.cfg.Bucket.String)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, nil, err
	}

	return share, func() {
		share.Umount()
		session.Logoff()
		conn.Close()
	}, nil
}

func (a *SMBAdapter) ListFiles(ctx context.Context, basePath string) ([]FileInfo, error) {
	share, cleanup, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	var files []FileInfo
	if err := walkShare(share, basePath, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func walkShare(share *smb2.Share, path string, files *[]FileInfo) error {
	entries, err := share.ReadDir(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fullPath := path + "/" + e.Name()
		if e.IsDir() {
			_ = walkShare(share, fullPath, files) // skip unreadable subdirs
		} else if isVideoFile(e.Name()) {
			*files = append(*files, FileInfo{
				Filename:    e.Name(),
				StoragePath: fullPath,
				SizeBytes:   e.Size(),
			})
		}
	}
	return nil
}

func (a *SMBAdapter) GetFile(_ context.Context, storagePath string) (io.ReadCloser, int64, error) {
	share, cleanup, err := a.connect()
	if err != nil {
		return nil, 0, err
	}

	f, err := share.Open(storagePath)
	if err != nil {
		cleanup()
		return nil, 0, err
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		cleanup()
		return nil, 0, err
	}

	return &smbFile{File: f, cleanup: cleanup}, stat.Size(), nil
}

func (a *SMBAdapter) WriteFile(_ context.Context, storagePath string, r io.Reader, _ int64) error {
	share, cleanup, err := a.connect()
	if err != nil {
		return err
	}
	defer cleanup()

	f, err := share.Create(storagePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func (a *SMBAdapter) DeleteFile(_ context.Context, storagePath string) error {
	share, cleanup, err := a.connect()
	if err != nil {
		return err
	}
	defer cleanup()
	return share.Remove(storagePath)
}

type smbFile struct {
	*smb2.File
	cleanup func()
}

func (f *smbFile) Close() error {
	err := f.File.Close()
	f.cleanup()
	return err
}
