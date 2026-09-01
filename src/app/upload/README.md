# Upload — Foundation Library

Generic file upload library. No DB table — purely foundation. Each domain creates its own `Uploader` with its own **MIME allowlist + size limit** and its own **storage backend** (local dir or S3-compatible like AWS S3 / Cloudflare R2 / MinIO). Validates by **MIME sniffing (512B) + extension cross-check** (strict — both must agree).

```
src/app/upload/
  config.go       — Config, LocalConfig, S3Config + Validate()
  upload.go       — UploadRequest, UploadResult, Uploader interface + strict MIME logic
  errors.go       — ErrBadRequest / ErrInvalidMIME / ErrTooLarge / ErrTransport / ErrTimeout + UploadError
  local.go        — filesystem backend (atomic CreateTemp + Rename)
  s3.go           — S3/R2 backend (aws-sdk-go-v2, BaseEndpoint + UsePathStyle)
  factory.go      — NewUploader / NewUploaderWithS3Client (test seam)
  uploadfakes/    — counterfeiter fake for tests
```

## Quick start

### 1. Create your uploader (per domain)

Each caller owns its instance — don't share one global uploader across domains if policies differ.

```go
import "github.com/ariesmaulana/ars-kit/src/app/upload"

// domain-local config — read from your own env, not global config.Config
avatarUploader, err := upload.NewUploader(upload.Config{
    AllowedMIMEs: []string{"image/jpeg", "image/png", "image/webp"},
    MaxSizeBytes: 2 * 1024 * 1024, // 2 MB
    Storage:      upload.StorageLocal,
    Local:        upload.LocalConfig{BaseDir: "./storage/avatars"},
})
if err != nil { log.Fatal(err) }
```

For **S3 / R2** the same `Config`, different `Storage`:

```go
avatarUploader, err := upload.NewUploader(upload.Config{
    AllowedMIMEs: []string{"image/jpeg", "image/png", "image/webp"},
    MaxSizeBytes: 2 * 1024 * 1024,
    Storage:      upload.StorageS3,
    S3: upload.S3Config{
        Bucket:          os.Getenv("R2_BUCKET"),            // e.g. "my-avatars"
        Region:          "auto",                            // R2 requires "auto"; S3 e.g. "ap-southeast-1"
        Endpoint:        os.Getenv("R2_ENDPOINT"),          // R2: https://<ACCOUNT_ID>.r2.cloudflarestorage.com
        AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
        SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
        Prefix:          "avatars/",                        // optional — keys become avatars/<xid>.jpg
        UsePathStyle:    true,                              // true for R2/MinIO, false for AWS S3
    },
})
```

`.env.example` for R2:

```env
R2_BUCKET=my-avatars
R2_ENDPOINT=https://<ACCOUNT_ID>.r2.cloudflarestorage.com
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
```

> AWS S3: leave `Endpoint` empty, set `Region` to e.g. `ap-southeast-1`, `UsePathStyle: false`.

### 2. Use in a handler (Echo) — avatar / photo profile example

This is how `src/app/user` would wire avatar upload. The lib does **not** handle `multipart` — the handler extracts primitives.

```go
// src/app/user/handler.go

func (h *Handler) UploadAvatar(c echo.Context) error {
    // 1. auth + permission check (domain responsibility)
    userID := getUserID(c) // from JWT

    // 2. extract file from multipart
    fh, err := c.FormFile("avatar")
    if err != nil {
        return c.JSON(400, map[string]string{"error": "avatar file is required"})
    }
    f, err := fh.Open()
    if err != nil {
        return c.JSON(500, map[string]string{"error": "cannot open file"})
    }
    defer f.Close()

    // 3. call foundation lib — pass io.Reader + original filename
    result, err := h.avatarUploader.Upload(c.Request().Context(), upload.UploadRequest{
        Reader:   f,
        Filename: fh.Filename,      // only basename matters; lib rejects "/" "\" ".."
        SizeHint: fh.Size,          // early reject if > MaxSizeBytes
        // KeyOverride: "avatars/123.jpg", // optional — auto xid.jpg if empty
    })
    if err != nil {
        if errors.Is(err, upload.ErrInvalidMIME) {
            return c.JSON(400, map[string]string{"error": "unsupported image type — use jpeg/png/webp"})
        }
        if errors.Is(err, upload.ErrTooLarge) {
            return c.JSON(400, map[string]string{"error": "avatar must be ≤ 2MB"})
        }
        if errors.Is(err, upload.ErrBadRequest) {
            return c.JSON(400, map[string]string{"error": err.Error()})
        }
        return c.JSON(500, map[string]string{"error": "upload failed"})
    }

    // 4. persist the key if your domain needs it (lib has no table)
    // e.g. UPDATE users SET avatar_key = $1 WHERE id = $2
    if err := h.userStorage.SetAvatarKey(c.Request().Context(), userID, result.Key); err != nil {
        // best-effort cleanup — don't leave orphan file
        _ = h.avatarUploader.Delete(context.Background(), result.Key)
        return c.JSON(500, map[string]string{"error": "could not save avatar"})
    }

    return c.JSON(200, map[string]any{
        "key":  result.Key,       // e.g. "3f9k2a1b.jpg" or "avatars/3f9k2a1b.jpg" (with prefix)
        "mime": result.MIME,      // "image/jpeg"
        "size": result.Size,
    })
}
```

### 3. Other operations

```go
// Delete — idempotent, NotFound = nil
if err := avatarUploader.Delete(ctx, key); err != nil {
    // only ErrBadRequest (bad key) or ErrTransport/ErrTimeout
}

// Presigned URL — S3/R2 only (local returns ErrBadRequest)
url, err := avatarUploader.PresignedURL(ctx, key, 15*time.Minute)
if err != nil {
    // ErrBadRequest if expiry not in [1m, 7d]
}
http.Redirect(w, r, url, http.StatusTemporaryRedirect) // 307
```

### 4. Error handling — same shape as `notification/email`

```go
import "errors"

// sentinel check
if errors.Is(err, upload.ErrInvalidMIME) { /* 400 */ }
if errors.Is(err, upload.ErrTooLarge)    { /* 400 */ }
if errors.Is(err, upload.ErrBadRequest)  { /* 400 */ }
if errors.Is(err, upload.ErrTransport)   { /* 500 */ }
if errors.Is(err, upload.ErrTimeout)     { /* 504 */ }

// typed check
var ue *upload.UploadError
if errors.As(err, &ue) {
    log.Warn().Str("backend", ue.Backend).Str("op", ue.Op).Err(ue).Msg("upload failed")
    // ue.Code is the sentinel (ErrInvalidMIME etc.)
}
```

### 5. Validation rules (strict)

- `Filename` must not contain `..` / `/` / `\` / null byte. Only extension is used — storage key is always lib-generated (`xid + ext`) unless `KeyOverride` is set.
- `KeyOverride` (optional) — allowed chars per segment: `[a-zA-Z0-9._-]` plus `/` as separator; no leading `/`, no `//`, no `..`. If override has no extension, original `ext` is appended; if it has one, it must match `Filename`'s ext.
- **MIME strict:** `http.DetectContentType` on first 512B must be in `AllowedMIMEs` **and** the extension's canonical MIME (via internal `extMIME` table) must also be in `AllowedMIMEs` and equal the sniffed MIME. Spoofed `photo.jpg` containing a PDF is rejected.
- **Size:** `SizeHint` early-rejects if `> MaxSizeBytes`; actual stream is `io.LimitReader(MaxSize+1)` + `ReadAll` — never loads more than `MaxSize+1` into memory. Caller must set `MaxSizeBytes > 0` (no implicit default).

Supported extensions in `extMIME`: `.jpg/.jpeg/.png/.gif/.webp/.svg/.bmp/.tiff/.pdf/.txt/.csv/.json/.xml/.zip/.gz/.tar/.mp4/.mov/.avi/.webm/.mp3/.wav/.ogg/.avif/.heic/.heif`. Unknown extensions are allowed only if the sniffed MIME itself is in the allowlist.

### 6. Wiring example — where to create the uploader

In `src/main.go` `buildApp()` or in `user.NewService` — keep it per-domain:

```go
// src/app/user/service.go or handler.go
type Service struct {
    storage        Storage
    avatarUploader upload.Uploader
}

func NewService(storage Storage, avatarUploader upload.Uploader) *Service {
    return &Service{storage: storage, avatarUploader: avatarUploader}
}

// in main.go
avatarUploader, _ := upload.NewUploader(upload.Config{
    AllowedMIMEs: []string{"image/jpeg", "image/png", "image/webp"},
    MaxSizeBytes: 2 << 20,
    Storage:      upload.StorageS3,
    S3: upload.S3Config{
        Bucket: os.Getenv("R2_BUCKET"), Region: "auto",
        Endpoint: os.Getenv("R2_ENDPOINT"),
        AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"),
        SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
        Prefix: "avatars/", UsePathStyle: true,
    },
})
userService := user.NewService(userStorage, avatarUploader)
```

For local dev, switch `Storage` to `StorageLocal` with `BaseDir: "./storage/avatars"` — no code change in handlers.

### 7. Testing your caller

Fake the interface — no real S3/filesystem needed:

```go
import "github.com/ariesmaulana/ars-kit/src/app/upload/uploadfakes"

fake := &uploadfakes.UploaderFake{}
fake.UploadReturns(&upload.UploadResult{Key: "avatars/xid.jpg", MIME: "image/jpeg", Size: 1234}, nil)

// inject into your service/handler under test
svc := user.NewService(storageFake, fake)

if fake.UploadCallCount() != 1 { t.Error("Upload not called") }
_, req := fake.UploadArgsForCall(0)
if req.Filename != "avatar.jpg" { t.Errorf("filename %q", req.Filename) }
```

For lib-level S3 tests, use `upload.NewUploaderWithS3Client(cfg, fakeS3, fakePresigner)` — see `upload_test.go` for a full fake `s3API` example.

### 8. Gotchas

- **R2 endpoint:** `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` — `ACCOUNT_ID` is the Cloudflare account ID, not the bucket name. `Region` is always `auto`.
- **Prefix:** normalized to `strings.Trim(prefix,"/")+"/"` — pass `"avatars"`, `"avatars/"`, or `"/avatars/"` all become `avatars/`.
- **Private by default:** no public URL is generated. Use `PresignedURL` (S3/R2) for reads; for local, serve via your own handler or static mount.
- **No DB row:** if you need to query by owner, store `result.Key` in your domain's table (e.g. `users.avatar_key`).
