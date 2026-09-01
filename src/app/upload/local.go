package upload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localBackend persists files to a filesystem directory.
type localBackend struct {
	baseDir string
}

func newLocalBackend(baseDir string) (*localBackend, error) {
	abs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve BaseDir: %w", err)
	}
	abs = filepath.Clean(abs)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create BaseDir: %w", err)
	}
	return &localBackend{baseDir: abs}, nil
}

func (b *localBackend) put(ctx context.Context, key string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return wrapError(ErrTimeout, "local", "upload", err)
	}
	fullPath := filepath.Join(b.baseDir, filepath.FromSlash(key))
	dir := filepath.Dir(fullPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}

	// Write atomically via temp file + rename.
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}
	if err := tmp.Close(); err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}
	if err := os.Rename(tmpName, fullPath); err != nil {
		return wrapError(ErrTransport, "local", "upload", err)
	}
	return nil
}

func (b *localBackend) delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return wrapError(ErrTimeout, "local", "delete", err)
	}
	fullPath := filepath.Join(b.baseDir, filepath.FromSlash(key))
	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return wrapError(ErrTransport, "local", "delete", err)
	}
	return nil
}
