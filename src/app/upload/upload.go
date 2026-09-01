package upload

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rs/xid"
)

// UploadRequest carries a single file to be persisted. The lib never depends
// on multipart directly — the caller extracts these primitives from HTTP.
type UploadRequest struct {
	Reader          io.Reader
	Filename        string
	SizeHint        int64
	ContentTypeHint string
	KeyOverride     string
}

// UploadResult is returned on successful persistence.
type UploadResult struct {
	Key       string
	Size      int64
	MIME      string
	Extension string
}

// Uploader is the foundation upload interface. Callers can fake it via
// uploadfakes or the in-memory gen_mock.
type Uploader interface {
	Upload(ctx context.Context, req UploadRequest) (*UploadResult, error)
	Delete(ctx context.Context, key string) error
	PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// uploader is the concrete implementation.
type uploader struct {
	allowed map[string]struct{}
	maxSize int64
	storage StorageKind

	local *localBackend
	s3    *s3Backend
}

// keyOverridePattern allows only safe characters in caller-provided keys.
// Segments are separated by "/"; each segment may contain alnum, dot,
// underscore, hyphen.
var keySegmentRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Upload implements Uploader.
func (u *uploader) Upload(ctx context.Context, req UploadRequest) (*UploadResult, error) {
	backend := string(u.storage)

	// --- validate trust boundary ---
	if req.Reader == nil {
		return nil, wrapError(ErrBadRequest, backend, "upload", fmt.Errorf("Reader is required"))
	}
	if strings.TrimSpace(req.Filename) == "" {
		return nil, wrapError(ErrBadRequest, backend, "upload", fmt.Errorf("Filename is required"))
	}
	if err := validateFilename(req.Filename); err != nil {
		return nil, wrapError(ErrBadRequest, backend, "upload", err)
	}
	if req.KeyOverride != "" {
		if err := validateKey(req.KeyOverride); err != nil {
			return nil, wrapError(ErrBadRequest, backend, "upload", fmt.Errorf("KeyOverride: %w", err))
		}
	}
	if req.SizeHint > u.maxSize && req.SizeHint >= 0 {
		return nil, wrapError(ErrTooLarge, backend, "upload", fmt.Errorf("size hint %d exceeds limit %d", req.SizeHint, u.maxSize))
	}
	if err := ctx.Err(); err != nil {
		return nil, wrapError(ErrTimeout, backend, "upload", err)
	}

	ext := strings.ToLower(filepath.Ext(req.Filename))

	// --- MIME sniffing (first 512 bytes) ---
	sniffBuf := make([]byte, 512)
	n, err := io.ReadFull(req.Reader, sniffBuf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, wrapError(ErrTransport, backend, "upload", fmt.Errorf("read sniff: %w", err))
	}
	sniffedFull := http.DetectContentType(sniffBuf[:n])
	mimeType := normalizeMIME(sniffedFull)

	// --- strict MIME + extension cross-check ---
	if err := u.validateMIME(mimeType, ext, backend); err != nil {
		return nil, err
	}

	// --- reconstruct stream + size enforcement ---
	reconstructed := io.MultiReader(bytes.NewReader(sniffBuf[:n]), req.Reader)
	limited := io.LimitReader(reconstructed, u.maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		if err == context.DeadlineExceeded || err == context.Canceled || ctx.Err() != nil {
			return nil, wrapError(ErrTimeout, backend, "upload", err)
		}
		return nil, wrapError(ErrTransport, backend, "upload", fmt.Errorf("read stream: %w", err))
	}
	if int64(len(data)) > u.maxSize {
		return nil, wrapError(ErrTooLarge, backend, "upload", fmt.Errorf("file size %d exceeds limit %d", len(data), u.maxSize))
	}

	// --- key generation ---
	var key string
	if req.KeyOverride != "" {
		key = sanitizeKey(req.KeyOverride)
		// Ensure override preserves extension consistency: if caller gave an ext,
		// it must match the sniff-derived ext or the original ext when known.
		overrideExt := strings.ToLower(filepath.Ext(key))
		if overrideExt != "" && ext != "" && overrideExt != ext {
			return nil, wrapError(ErrBadRequest, backend, "upload", fmt.Errorf("KeyOverride extension %q does not match Filename extension %q", overrideExt, ext))
		}
		if overrideExt == "" && ext != "" {
			key = key + ext
		}
	} else {
		key = xid.New().String() + ext
	}

	// --- persist ---
	switch u.storage {
	case StorageLocal:
		if err := u.local.put(ctx, key, data); err != nil {
			return nil, err
		}
	case StorageS3:
		if err := u.s3.put(ctx, key, mimeType, data); err != nil {
			return nil, err
		}
	default:
		return nil, wrapError(ErrBadRequest, backend, "upload", fmt.Errorf("unknown storage"))
	}

	return &UploadResult{
		Key:       key,
		Size:      int64(len(data)),
		MIME:      mimeType,
		Extension: ext,
	}, nil
}

func (u *uploader) Delete(ctx context.Context, key string) error {
	backend := string(u.storage)
	if strings.TrimSpace(key) == "" {
		return wrapError(ErrBadRequest, backend, "delete", fmt.Errorf("key is required"))
	}
	if err := validateKey(key); err != nil {
		return wrapError(ErrBadRequest, backend, "delete", err)
	}
	if err := ctx.Err(); err != nil {
		return wrapError(ErrTimeout, backend, "delete", err)
	}
	cleaned := sanitizeKey(key)
	switch u.storage {
	case StorageLocal:
		return u.local.delete(ctx, cleaned)
	case StorageS3:
		return u.s3.delete(ctx, cleaned)
	default:
		return wrapError(ErrBadRequest, backend, "delete", fmt.Errorf("unknown storage"))
	}
}

func (u *uploader) PresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if u.storage != StorageS3 {
		return "", wrapError(ErrBadRequest, string(u.storage), "presign", fmt.Errorf("presign not supported for %s storage", u.storage))
	}
	if strings.TrimSpace(key) == "" {
		return "", wrapError(ErrBadRequest, "s3", "presign", fmt.Errorf("key is required"))
	}
	if err := validateKey(key); err != nil {
		return "", wrapError(ErrBadRequest, "s3", "presign", err)
	}
	if err := ctx.Err(); err != nil {
		return "", wrapError(ErrTimeout, "s3", "presign", err)
	}
	return u.s3.presignedURL(ctx, sanitizeKey(key), expiry)
}

// validateMIME enforces strict allowlist + extension cross-check.
func (u *uploader) validateMIME(mimeType, ext, backend string) error {
	mimeType = strings.ToLower(mimeType)
	if _, ok := u.allowed[mimeType]; !ok {
		return wrapError(ErrInvalidMIME, backend, "upload", fmt.Errorf("MIME %q not in allowlist", mimeType))
	}
	if ext == "" {
		return nil
	}
	extMime, known := extMIME[ext]
	if !known {
		// Unknown extension — allow only if caller explicitly allows octet-stream
		// or the sniffed type itself is allowed (already checked above).
		return nil
	}
	extMime = strings.ToLower(extMime)
	if _, ok := u.allowed[extMime]; !ok {
		return wrapError(ErrInvalidMIME, backend, "upload", fmt.Errorf("extension %q maps to MIME %q not in allowlist", ext, extMime))
	}
	// When both are known, they must be compatible. We treat exact match as
	// required except for jpeg/jpg alias which extMIME already normalizes.
	if mimeType != extMime {
		// Allow text/plain sniff for .csv/.json etc? No — strict per plan.
		return wrapError(ErrInvalidMIME, backend, "upload", fmt.Errorf("MIME mismatch: sniffed %q vs extension %q (%q)", mimeType, ext, extMime))
	}
	return nil
}

func normalizeMIME(m string) string {
	// "image/jpeg; charset=utf-8" -> "image/jpeg"
	if idx := strings.Index(m, ";"); idx >= 0 {
		m = m[:idx]
	}
	return strings.ToLower(strings.TrimSpace(m))
}

func validateFilename(name string) error {
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename contains null byte")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("filename must not contain '..'")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return fmt.Errorf("filename must not contain path separators")
	}
	return nil
}

func validateKey(key string) error {
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("key contains null byte")
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("key must not start with '/'")
	}
	if strings.Contains(key, "..") {
		return fmt.Errorf("key must not contain '..'")
	}
	if strings.Contains(key, `\`) {
		return fmt.Errorf("key must not contain '\\'")
	}
	segments := strings.Split(key, "/")
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("key must not contain empty segment '//'")
		}
		if seg == "." || seg == ".." {
			return fmt.Errorf("key segment %q not allowed", seg)
		}
		if !keySegmentRE.MatchString(seg) {
			return fmt.Errorf("key segment %q contains invalid characters", seg)
		}
	}
	return nil
}

func sanitizeKey(key string) string {
	// Normalize: trim spaces, clean, keep forward slashes.
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "/")
	return key
}

// extMIME maps common extensions (lowercase, with dot) to their canonical MIME.
var extMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".pdf":  "application/pdf",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".json": "application/json",
	".xml":  "application/xml",
	".zip":  "application/zip",
	".gz":   "application/gzip",
	".tar":  "application/x-tar",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".webm": "video/webm",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".avif": "image/avif",
	".heic": "image/heic",
	".heif": "image/heif",
}
