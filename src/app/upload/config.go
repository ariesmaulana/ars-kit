package upload

import (
	"fmt"
	"strings"
)

// StorageKind identifies the persistence backend.
type StorageKind string

const (
	StorageLocal StorageKind = "local"
	StorageS3    StorageKind = "s3"
)

// Config holds per-caller upload configuration. Each domain constructs its own
// Uploader with its own allowlist and limits.
type Config struct {
	AllowedMIMEs []string
	MaxSizeBytes int64

	Storage StorageKind
	Local   LocalConfig
	S3      S3Config
}

// LocalConfig configures filesystem storage.
type LocalConfig struct {
	BaseDir string
}

// S3Config configures S3-compatible storage (AWS S3, Cloudflare R2, MinIO).
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Prefix          string
	UsePathStyle    bool
}

// normalizedPrefix returns Prefix trimmed and suffixed with "/" when non-empty.
func (c S3Config) normalizedPrefix() string {
	p := strings.Trim(c.Prefix, "/")
	if p == "" {
		return ""
	}
	return p + "/"
}

func (c Config) validate() error {
	if len(c.AllowedMIMEs) == 0 {
		return fmt.Errorf("AllowedMIMEs must not be empty")
	}
	for _, m := range c.AllowedMIMEs {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "" || !strings.Contains(m, "/") {
			return fmt.Errorf("invalid MIME %q: must contain '/'", m)
		}
	}
	if c.MaxSizeBytes <= 0 {
		return fmt.Errorf("MaxSizeBytes must be positive, got %d", c.MaxSizeBytes)
	}
	switch c.Storage {
	case StorageLocal:
		if strings.TrimSpace(c.Local.BaseDir) == "" {
			return fmt.Errorf("Local.BaseDir is required for local storage")
		}
	case StorageS3:
		if strings.TrimSpace(c.S3.Bucket) == "" {
			return fmt.Errorf("S3.Bucket is required for s3 storage")
		}
		if strings.TrimSpace(c.S3.Region) == "" {
			return fmt.Errorf("S3.Region is required for s3 storage")
		}
		if strings.TrimSpace(c.S3.AccessKeyID) == "" {
			return fmt.Errorf("S3.AccessKeyID is required for s3 storage")
		}
		if strings.TrimSpace(c.S3.SecretAccessKey) == "" {
			return fmt.Errorf("S3.SecretAccessKey is required for s3 storage")
		}
	default:
		return fmt.Errorf("unknown Storage %q: must be %q or %q", c.Storage, StorageLocal, StorageS3)
	}
	return nil
}
