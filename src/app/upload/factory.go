package upload

import (
	"fmt"
	"strings"
)

// NewUploader constructs an Uploader for the given config. The caller defines
// its own allowlist and size limit — each domain owns its uploader instance.
func NewUploader(cfg Config) (Uploader, error) {
	if err := cfg.validate(); err != nil {
		return nil, wrapError(ErrBadRequest, "", "new_uploader", err)
	}

	allowed := make(map[string]struct{}, len(cfg.AllowedMIMEs))
	for _, m := range cfg.AllowedMIMEs {
		allowed[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}

	u := &uploader{
		allowed: allowed,
		maxSize: cfg.MaxSizeBytes,
		storage: cfg.Storage,
	}

	switch cfg.Storage {
	case StorageLocal:
		lb, err := newLocalBackend(cfg.Local.BaseDir)
		if err != nil {
			return nil, wrapError(ErrBadRequest, "local", "new_uploader", err)
		}
		u.local = lb
	case StorageS3:
		sb, err := newS3Backend(cfg.S3)
		if err != nil {
			return nil, wrapError(ErrTransport, "s3", "new_uploader", err)
		}
		u.s3 = sb
	default:
		return nil, wrapError(ErrBadRequest, "", "new_uploader", fmt.Errorf("unknown storage %q", cfg.Storage))
	}

	return u, nil
}

// NewUploaderWithS3Client is a test seam that injects a fake S3 client.
// It validates the config identically to NewUploader but uses the provided
// client instead of building a real one. Only valid when Storage == "s3".
func NewUploaderWithS3Client(cfg Config, client s3API, presigner s3Presigner) (Uploader, error) {
	if cfg.Storage != StorageS3 {
		return nil, wrapError(ErrBadRequest, "", "new_uploader", fmt.Errorf("NewUploaderWithS3Client only supports s3 storage"))
	}
	if err := cfg.validate(); err != nil {
		return nil, wrapError(ErrBadRequest, "", "new_uploader", err)
	}
	allowed := make(map[string]struct{}, len(cfg.AllowedMIMEs))
	for _, m := range cfg.AllowedMIMEs {
		allowed[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}
	sb := newS3BackendWithClient(cfg.S3, client, presigner)
	return &uploader{
		allowed: allowed,
		maxSize: cfg.MaxSizeBytes,
		storage: StorageS3,
		s3:      sb,
	}, nil
}
