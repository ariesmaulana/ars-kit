package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ── helpers ────────────────────────────────────────────────────────────────

func localCfg(dir string, allowed []string, max int64) Config {
	return Config{
		AllowedMIMEs: allowed,
		MaxSizeBytes: max,
		Storage:      StorageLocal,
		Local:        LocalConfig{BaseDir: dir},
	}
}

func s3Cfg(allowed []string, max int64) Config {
	return Config{
		AllowedMIMEs: allowed,
		MaxSizeBytes: max,
		Storage:      StorageS3,
		S3: S3Config{
			Bucket:          "test-bucket",
			Region:          "us-east-1",
			AccessKeyID:     "akid",
			SecretAccessKey: "secret",
			Prefix:          "prefix/",
		},
	}
}

// minimal valid file headers for DetectContentType
var (
	jpegHeader = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	pngHeader  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}
	pdfHeader  = []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4', '\n', '%', 0xE2, 0xE3, 0xCF, 0xD3}
)

func padded(header []byte, total int) []byte {
	if len(header) >= total {
		return header[:total]
	}
	b := make([]byte, total)
	copy(b, header)
	for i := len(header); i < total; i++ {
		b[i] = 'a'
	}
	return b
}

// ── config validation ───────────────────────────────────────────────────

func TestNewUploader_Validation(t *testing.T) {
	tmp := t.TempDir()
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{"empty allowlist", Config{AllowedMIMEs: nil, MaxSizeBytes: 100, Storage: StorageLocal, Local: LocalConfig{BaseDir: tmp}}, "AllowedMIMEs"},
		{"zero max", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: 0, Storage: StorageLocal, Local: LocalConfig{BaseDir: tmp}}, "MaxSizeBytes"},
		{"negative max", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: -1, Storage: StorageLocal, Local: LocalConfig{BaseDir: tmp}}, "MaxSizeBytes"},
		{"bad mime", Config{AllowedMIMEs: []string{"notamime"}, MaxSizeBytes: 100, Storage: StorageLocal, Local: LocalConfig{BaseDir: tmp}}, "invalid MIME"},
		{"unknown storage", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: 100, Storage: "bad"}, "unknown Storage"},
		{"local missing BaseDir", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: 100, Storage: StorageLocal}, "BaseDir"},
		{"s3 missing bucket", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: 100, Storage: StorageS3, S3: S3Config{Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "s"}}, "Bucket"},
		{"s3 missing region", Config{AllowedMIMEs: []string{"image/jpeg"}, MaxSizeBytes: 100, Storage: StorageS3, S3: S3Config{Bucket: "b", AccessKeyID: "a", SecretAccessKey: "s"}}, "Region"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewUploader(tc.cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
			var ue *UploadError
			if !errors.As(err, &ue) {
				t.Fatalf("error not *UploadError: %T", err)
			}
			if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("expected ErrBadRequest, got %v", err)
			}
		})
	}
}

// ── local upload happy paths ────────────────────────────────────────────

func TestUpload_Local_Success_JPEG(t *testing.T) {
	dir := t.TempDir()
	u, err := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	if err != nil {
		t.Fatalf("NewUploader: %v", err)
	}
	data := padded(jpegHeader, 100)
	res, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "photo.jpg",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q, want image/jpeg", res.MIME)
	}
	if res.Extension != ".jpg" {
		t.Errorf("Extension = %q, want .jpg", res.Extension)
	}
	if res.Size != int64(len(data)) {
		t.Errorf("Size = %d, want %d", res.Size, len(data))
	}
	if !strings.HasSuffix(res.Key, ".jpg") {
		t.Errorf("Key %q should end with .jpg", res.Key)
	}
	// verify file exists
	path := filepath.Join(dir, filepath.FromSlash(res.Key))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("file content mismatch")
	}
}

func TestUpload_Local_Success_PNG(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/png"}, 1024))
	data := padded(pngHeader, 200)
	res, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "img.png",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.MIME != "image/png" {
		t.Errorf("MIME = %q, want image/png", res.MIME)
	}
}

func TestUpload_Local_Success_PDF(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"application/pdf"}, 2048))
	data := padded(pdfHeader, 300)
	res, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "doc.pdf",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.MIME != "application/pdf" {
		t.Errorf("MIME = %q, want application/pdf", res.MIME)
	}
}

// ── MIME strict validation ──────────────────────────────────────────────

func TestUpload_MIME_Mismatch_ExtensionSpoof(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg", "image/png"}, 2048))
	// JPEG bytes but .png extension -> should fail
	data := padded(jpegHeader, 100)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "evil.png",
	})
	if err == nil {
		t.Fatal("expected MIME mismatch error, got nil")
	}
	if !errors.Is(err, ErrInvalidMIME) {
		t.Fatalf("expected ErrInvalidMIME, got %v", err)
	}
}

func TestUpload_MIME_NotInAllowlist(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/png"}, 2048))
	data := padded(jpegHeader, 100)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "photo.jpg",
	})
	if !errors.Is(err, ErrInvalidMIME) {
		t.Fatalf("expected ErrInvalidMIME, got %v", err)
	}
}

func TestUpload_ExtensionNotInAllowlist(t *testing.T) {
	dir := t.TempDir()
	// allow image/jpeg but not image/png; .png ext maps to image/png which is not allowed
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(pngHeader, 100)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "img.png",
	})
	if !errors.Is(err, ErrInvalidMIME) {
		t.Fatalf("expected ErrInvalidMIME for extension not in allowlist, got %v", err)
	}
}

// ── size enforcement ────────────────────────────────────────────────────

func TestUpload_SizeHintEarlyReject(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 10))
	data := padded(jpegHeader, 5)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "a.jpg",
		SizeHint: 100, // exceeds 10
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestUpload_TooLarge_Stream(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 10))
	// 20 bytes > 10 limit
	data := padded(jpegHeader, 20)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "a.jpg",
	})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestUpload_ExactLimit_OK(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 10))
	data := padded(jpegHeader, 10)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:   bytes.NewReader(data),
		Filename: "a.jpg",
	})
	if err != nil {
		t.Fatalf("expected success at exact limit, got %v", err)
	}
}

// ── key override ────────────────────────────────────────────────────────

func TestUpload_KeyOverride_Success(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(jpegHeader, 50)
	res, err := u.Upload(context.Background(), UploadRequest{
		Reader:      bytes.NewReader(data),
		Filename:    "photo.jpg",
		KeyOverride: "my/custom_key",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	// key override without ext gets ext appended
	if res.Key != "my/custom_key.jpg" {
		t.Errorf("Key = %q, want my/custom_key.jpg", res.Key)
	}
	path := filepath.Join(dir, filepath.FromSlash(res.Key))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not found at %q: %v", path, err)
	}
}

func TestUpload_KeyOverride_WithMatchingExt(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(jpegHeader, 50)
	res, err := u.Upload(context.Background(), UploadRequest{
		Reader:      bytes.NewReader(data),
		Filename:    "photo.jpg",
		KeyOverride: "my/file.jpg",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.Key != "my/file.jpg" {
		t.Errorf("Key = %q, want my/file.jpg", res.Key)
	}
}

func TestUpload_KeyOverride_MismatchExt(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(jpegHeader, 50)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:      bytes.NewReader(data),
		Filename:    "photo.jpg",
		KeyOverride: "my/file.png",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for ext mismatch, got %v", err)
	}
}

func TestUpload_KeyOverride_InvalidChars(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(jpegHeader, 50)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:      bytes.NewReader(data),
		Filename:    "photo.jpg",
		KeyOverride: "../evil.jpg",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for traversal, got %v", err)
	}
}

func TestUpload_KeyOverride_EmptySegment(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 2048))
	data := padded(jpegHeader, 50)
	_, err := u.Upload(context.Background(), UploadRequest{
		Reader:      bytes.NewReader(data),
		Filename:    "photo.jpg",
		KeyOverride: "a//b.jpg",
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for empty segment, got %v", err)
	}
}

// ── trust boundary: filename, reader ───────────────────────────────────

func TestUpload_NilReader(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 100))
	_, err := u.Upload(context.Background(), UploadRequest{Filename: "a.jpg"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestUpload_EmptyFilename(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 100))
	_, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader([]byte("hi")), Filename: ""})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestUpload_FilenameWithPathSeparator(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 100))
	_, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(padded(jpegHeader, 10)), Filename: "a/b.jpg"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestUpload_FilenameWithDotDot(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 100))
	_, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(padded(jpegHeader, 10)), Filename: "..evil.jpg"})
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestUpload_ContextCancelled(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 100))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := u.Upload(ctx, UploadRequest{Reader: bytes.NewReader(padded(jpegHeader, 10)), Filename: "a.jpg"})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

// ── delete ──────────────────────────────────────────────────────────────

func TestDelete_Local_Success(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	data := padded(jpegHeader, 50)
	res, _ := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(data), Filename: "a.jpg"})
	if err := u.Delete(context.Background(), res.Key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	path := filepath.Join(dir, filepath.FromSlash(res.Key))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, stat err = %v", err)
	}
}

func TestDelete_Local_Idempotent_NotExist(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	if err := u.Delete(context.Background(), "nonexistent.jpg"); err != nil {
		t.Fatalf("expected nil for not-exist delete, got %v", err)
	}
}

func TestDelete_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	if err := u.Delete(context.Background(), "../evil.jpg"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if err := u.Delete(context.Background(), ""); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for empty key, got %v", err)
	}
}

// ── presigned URL ───────────────────────────────────────────────────────

func TestPresignedURL_Local_NotSupported(t *testing.T) {
	dir := t.TempDir()
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	_, err := u.PresignedURL(context.Background(), "a.jpg", time.Minute*5)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for local presign, got %v", err)
	}
}

// ── S3 with fakes ───────────────────────────────────────────────────────

type fakeS3 struct {
	putCalled    bool
	putKey       string
	putMIME      string
	putData      []byte
	deleteCalled bool
	deleteKey    string
	presignURL   string
	presignErr   error
	putErr       error
}

func (f *fakeS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	f.putCalled = true
	if in.Key != nil {
		f.putKey = *in.Key
	}
	if in.ContentType != nil {
		f.putMIME = *in.ContentType
	}
	if in.Body != nil {
		b, _ := io.ReadAll(in.Body)
		f.putData = b
	}
	return &s3.PutObjectOutput{}, nil
}
func (f *fakeS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleteCalled = true
	if in.Key != nil {
		f.deleteKey = *in.Key
	}
	return &s3.DeleteObjectOutput{}, nil
}
func (f *fakeS3) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, nil
}

type fakePresigner struct {
	url string
	err error
}

func (f *fakePresigner) PresignGetObject(_ context.Context, in *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4PresignedURL, error) {
	if f.err != nil {
		return nil, f.err
	}
	// honour expiry: just return URL
	return &v4PresignedURL{URL: f.url}, nil
}

func TestUpload_S3_Success(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{url: "https://presigned.example.com/file"}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	cfg.S3.Prefix = "avatars/"
	u, err := NewUploaderWithS3Client(cfg, fake, pres)
	if err != nil {
		t.Fatalf("NewUploaderWithS3Client: %v", err)
	}
	data := padded(jpegHeader, 80)
	res, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(data), Filename: "photo.jpg"})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q", res.MIME)
	}
	if !fake.putCalled {
		t.Error("PutObject not called")
	}
	if !strings.HasPrefix(fake.putKey, "avatars/") {
		t.Errorf("putKey %q should have prefix avatars/", fake.putKey)
	}
	if !strings.HasSuffix(fake.putKey, ".jpg") {
		t.Errorf("putKey %q should end with .jpg", fake.putKey)
	}
	if fake.putMIME != "image/jpeg" {
		t.Errorf("putMIME = %q", fake.putMIME)
	}
}

func TestUpload_S3_Prefix_Normalization(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	cfg.S3.Prefix = "my/prefix" // no trailing slash
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	data := padded(jpegHeader, 10)
	_, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(data), Filename: "a.jpg"})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !strings.HasPrefix(fake.putKey, "my/prefix/") {
		t.Errorf("prefix normalization failed: %q", fake.putKey)
	}
}

func TestUpload_S3_TransportError(t *testing.T) {
	fake := &fakeS3{putErr: errors.New("network down")}
	pres := &fakePresigner{}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	data := padded(jpegHeader, 10)
	_, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(data), Filename: "a.jpg"})
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("expected ErrTransport, got %v", err)
	}
}

func TestS3_Delete_Success(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	cfg.S3.Prefix = "docs/"
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	if err := u.Delete(context.Background(), "myfile.jpg"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !fake.deleteCalled {
		t.Error("DeleteObject not called")
	}
	if fake.deleteKey != "docs/myfile.jpg" {
		t.Errorf("deleteKey = %q, want docs/myfile.jpg", fake.deleteKey)
	}
}

func TestS3_PresignedURL_Success(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{url: "https://example.com/presigned?sig=abc"}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	url, err := u.PresignedURL(context.Background(), "a.jpg", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignedURL: %v", err)
	}
	if url != "https://example.com/presigned?sig=abc" {
		t.Errorf("url = %q", url)
	}
}

func TestS3_PresignedURL_BadExpiry(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{url: "https://example.com"}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	_, err := u.PresignedURL(context.Background(), "a.jpg", 30*time.Second)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for short expiry, got %v", err)
	}
	_, err = u.PresignedURL(context.Background(), "a.jpg", 8*24*time.Hour)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for long expiry, got %v", err)
	}
}

func TestS3_PresignedURL_CancelledContext(t *testing.T) {
	fake := &fakeS3{}
	pres := &fakePresigner{url: "https://example.com"}
	cfg := s3Cfg([]string{"image/jpeg"}, 1024)
	u, _ := NewUploaderWithS3Client(cfg, fake, pres)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := u.PresignedURL(ctx, "a.jpg", 5*time.Minute)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

// ── error wrapping ──────────────────────────────────────────────────────

func TestErrorWrapping(t *testing.T) {
	err := wrapError(ErrInvalidMIME, "s3", "upload", errors.New("bad mime"))
	if !errors.Is(err, ErrInvalidMIME) {
		t.Error("errors.Is should match ErrInvalidMIME")
	}
	var ue *UploadError
	if !errors.As(err, &ue) {
		t.Fatal("should be *UploadError")
	}
	if ue.Code != ErrInvalidMIME {
		t.Errorf("Code = %v, want ErrInvalidMIME", ue.Code)
	}
	if ue.Backend != "s3" || ue.Op != "upload" {
		t.Errorf("Backend/Op = %q/%q", ue.Backend, ue.Op)
	}
}

// ── unknown extension path ──────────────────────────────────────────────

func TestUpload_UnknownExtension_AllowedWhenMIMEAllowed(t *testing.T) {
	dir := t.TempDir()
	// allow image/jpeg; use .xyz unknown ext with jpeg bytes
	// extMIME does not know .xyz, so only sniffed MIME check applies
	u, _ := NewUploader(localCfg(dir, []string{"image/jpeg"}, 1024))
	data := padded(jpegHeader, 50)
	res, err := u.Upload(context.Background(), UploadRequest{Reader: bytes.NewReader(data), Filename: "file.xyz"})
	if err != nil {
		t.Fatalf("expected success for unknown ext with allowed MIME, got %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Errorf("MIME = %q", res.MIME)
	}
}
