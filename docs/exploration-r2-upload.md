# Exploration: Upload File ke Cloudflare R2 — Rencana Implementasi MVP

Task: `t_28fc7de6` · Branch: `feat/r2-upload-exploration` · Status: **grilling selesai, shared understanding tercapai — menunggu review user**

Dokumen ini merangkum seluruh keputusan grilling (Q1–Q7) dan rencana implementasi
MVP integrasi upload file ke Cloudflare R2 untuk ars-kit (Go + Echo + PostgreSQL).
Exploration selesai — dokumen ini adalah **task plan** untuk direview user sebelum
implementasi dimulai.

---

## 1. Keputusan Grilling (Q1–Q7)

### Q1 — Kasus penggunaan / jenis file ✅ SETTLED
**Jawaban user:** *"Kita buat general, nanti kalau ada fitur upload avatar untuk image, kalau video ya video, jadi depends konteks uploadnya."*

- **Upload generic** — satu mekanisme upload untuk semua jenis file user.
- Konteks upload (dan kebijakannya) didefinisikan oleh domain pemakai, bukan enum bawaan lib.

### Q2 — Kategori & kebijakan per kategori ✅ SETTLED
**Jawaban user:** *"Upload adalah satu support lib configurable by caller. Jadi nanti di tiap domain yang pakai, di inisialisasinya define jenis file dan limit size. Pengecekan file wajib dari mime type bukan cuma extension."*

- **Upload = support library (foundation module) yang dikonfigurasi caller/domain saat init.**
- Tiap domain mendaftarkan konteks dengan **allowlist MIME type + size limit sendiri** (contoh: `avatar` = jpeg/png/webp ≤ 2MB; `receipt` = pdf/jpeg/png ≤ 20MB).
- **Validasi tipe file WAJIB MIME-based** (sniff magic bytes), bukan extension — extension bisa dipalsukan.

### Q3 — Bucket & key layout ✅ SETTLED
**Jawaban user:** **'A'**

- **Satu bucket** (satu env var `R2_BUCKET`) + **prefix per konteks** sebagai pemisah — bukan multi-bucket per konteks. Kebijakan per konteks (cache/public/lifecycle) di R2 bisa diisolasi per-prefix.
- **Format key: `<context>/<ownerID>/<xid><ext>`**
  - `context` = konteks terdaftar (mis. `avatar`, `receipt`)
  - `ownerID` = pemilik file (untuk audit & ownership check)
  - `xid` = id unik (dipakai juga sebagai PK tabel, konsisten dengan pola repo)
  - `ext` = extension **diturunkan dari hasil sniff MIME**, BUKAN dari nama file user

### Q4 — Skema tabel `file_uploads` ✅ SETTLED
**Jawaban user:** *'Iya'* (opsi **B — tabel penuh**); `id` = xid text (setuju); **`original_name` DISIMPAN** (dipakai `Content-Disposition` saat download).

- Postgres sebagai query layer untuk ownership check & listing (repo Postgres-centric); mime/size terbaca tanpa `HEAD` R2; original filename punya rumah; fondasi quota/audit/cleanup.
- `owner_id` plain integer tanpa FK (ikut gaya repo).

### Q5 — Upload flow & mekanisme download ✅ SETTLED
**Jawaban user:** **'A'** (setuju rekomendasi).

- **Upload: streaming multipart** — baca 512 byte pertama → sniff MIME → validasi allowlist + size policy → stream sisa ke R2 sambil hitung byte, abort bila melewati `MaxSize`. Memory konstan.
- **Bypass `BodyLimit("1M")` global** untuk route upload (via `BodyLimitConfig.Skipper`); enforce ukuran di layer service (policy per konteks).
- **Download: presigned URL R2** — server verifikasi ownership (`owner_id` vs user dari token) → redirect ke presigned GET URL expiry pendek. File tidak di-proxy lewat server.
- **Semua konteks DEFAULT PRIVATE** — tidak ada public access di MVP.

### Q6 — Security: sniffing MIME & titik enforce permission ✅ SETTLED
**Jawaban user:** setuju rekomendasi pi (Bagian 1 = B, Bagian 2 = A; asumsi default diterima).

- **Sniffing MIME = `gabriel-vasile/mimetype`** — 1 dependency pure-Go; coverage luas (mp4/mov/avif/heic yang gagal di stdlib `http.DetectContentType`); **extension lookup built-in**; pembeda text vs binary lebih baik. Ini security boundary, jadi lib yang matang sepadan.
- **Lib murni teknis** — lib hanya enforce ownership + policy (allowlist MIME + size). Cek permission bisnis ("user berhak upload ke konteks X") dilakukan **domain pemanggil** sebelum memanggil lib (konsisten pola `permission` module; konstanta di `const.go` domain).
- **Asumsi default:** sniffed MIME ∉ allowlist → **tolak** (4xx), jangan rewrite; `Content-Type` header client **tidak dipercaya** (sniff = sumber kebenaran).

### Q7 — Paket desain MVP ✅ SETTLED (grilling selesai)
**Jawaban user:** **'Ok'** — seluruh paket disetujui.

- Struktur lib: **`src/app/upload/`** (mengikuti pola `permission`).
- API kontrak: **Register / Store / DownloadURL / Delete**.
- Asumsi default: presigned expiry **15 menit** (bisa di-override per panggilan); **Delete termasuk MVP**; **tanpa listing/quota/public flag/rename**.

### Di-fold ke rencana (fakta / keputusan teknis, bukan pertanyaan)

| Item | Keputusan |
|---|---|
| **Env vars (Q7)** | `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET`. Endpoint S3 derivable: `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`. `R2_PUBLIC_URL` **tidak** dibutuhkan (semua konteks private). |
| **Go client (Q10)** | **`minio-go/v7`** — R2 S3-compatible; client paling ringkas (1 module; `PutObject` streaming, `PresignedGetObject`, `RemoveObject`; path-style URL cocok custom endpoint R2). `aws-sdk-go-v2` terlalu berat & verbose untuk put+presign+delete MVP. |

### Dibuang / ditunda dari MVP

- **Rate limiting upload per-konteks/per-user (pola LoginThrottle) → DEFERRED.** Global limiter 20 req/s burst 40 sudah menutupi route upload; hardening ditambah post-MVP kalau ada bukti kebutuhan.
- Public access / `public` flag per-konteks.
- Listing, quota, rename, virus-scan, dedupe.
- Route upload generic bawaan lib — tiap domain pasang route sendiri + cek permission lalu panggil lib.

---

## 2. Struktur File

Modul baru `src/app/upload/`, mengikuti pola foundation module `permission`:

```
src/app/upload/
├── service.go              # Brain: Register/Store/DownloadURL/Delete + policy check + R2 ops
├── service_interface.go    # Kontrak: Context, Service interface, named input/output types
├── storage.go              # Repository SQL untuk file_uploads (pgx)
├── storage_interface.go    # Kontrak storage + kategorisasi error (ErrTypeNotFound, ...)
├── object_store.go         # Adapter tipis minio-go di balik interface ObjectStore (seam utk test)
├── migrate.go              # //go:embed sql/*.sql → var Migrations embed.FS
├── sql/
│   └── 20260101_000001_create_file_uploads.sql
├── gen_mock.go             # counterfeiter fakes (ObjectStore, Storage)
├── fakes/
│   └── fake_object_store.go
├── service_test.go         # suite-based, real DB (file_uploads) + fake ObjectStore
├── storage_test.go
├── setup_test.go
└── testing_helper_test.go
```

Perubahan file yang sudah ada:

```
config/config.go            # + struct fields R2* + parse dari env
.env.example                # + R2_* vars
database/migrator.go        # + domain {Name: "upload", TableName: "goose_db_version_upload", FS: upload.Migrations}
src/main.go                 # + wiring: bangun minio client + upload service (foundation section), tambah field App.UploadService
go.mod / go.sum             # + github.com/minio/minio-go/v7, github.com/gabriel-vasile/mimetype
```

> Catatan: modul `upload` tidak punya `const.go`/`handler.go` — lib murni teknis,
> tidak punya permission sendiri dan tidak mendaftarkan route (keputusan Q6/Q7).

---

## 3. API Kontrak

### 3.1 Konfigurasi konteks (dipanggil domain pemakai saat init)

```go
// service_interface.go
type Context struct {
    Name      string   // prefix key: "avatar" → avatar/<ownerID>/<xid><ext>
    MIMEAllow []string // allowlist hasil sniff, mis. {"image/jpeg", "image/png", "image/webp"}
    MaxSize   int64    // byte, mis. 2 << 20 (2MB)
}

type Config struct {
    Endpoint        string // https://<ACCOUNT_ID>.r2.cloudflarestorage.com
    AccessKeyID     string
    SecretAccessKey string
    Bucket          string
    Region          string // R2: selalu "auto"
}
```

### 3.2 Service API (dipanggil handler domain SETELAH cek permission sendiri)

```go
type Service interface {
    // Register mendaftarkan konteks upload. Dipanggil sekali saat startup
    // (buildApp). Nama duplikat / MIMEAllow kosong / MaxSize <= 0 → error.
    Register(ctx context.Context, cfg Context) error

    // Store: sniff 512B pertama → validasi MIME allowlist + MaxSize →
    // stream sisa ke R2 (hitung byte, abort bila lewat MaxSize) →
    // insert baris file_uploads. Return id (xid) + metadata tersimpan.
    Store(ctx context.Context, input *StoreInput) (*StoreOutput, error)

    // DownloadURL: cek ownership (owner_id == viewer_id) → presigned GET URL.
    // expiry default 15 menit, bisa di-override.
    DownloadURL(ctx context.Context, input *DownloadURLInput) (*DownloadURLOutput, error)

    // Delete: cek ownership → hapus object R2 → hapus baris. Termasuk MVP.
    Delete(ctx context.Context, input *DeleteInput) *DeleteOutput
}

type StoreInput struct {
    TraceId      string
    Context      string   // nama konteks terdaftar
    OwnerID      int      // users.id
    File         io.Reader // stream multipart (sudah dibuka handler)
    OriginalName string   // nama file asli user → kolom original_name
}

type StoreOutput struct {
    Success   bool
    Message   string
    ErrorCode string // validation | forbidden | not_found | internal
    ID        string // xid
    MIME      string // hasil sniff
    Size      int64
    Key       string
}

type DownloadURLInput struct {
    TraceId  string
    Context  string
    ID       string // xid
    ViewerID int    // user dari token
    Expiry   time.Duration // 0 → default 15 menit
}

type DownloadURLOutput struct {
    Success   bool
    Message   string
    ErrorCode string
    URL       string // presigned GET (redirect target handler)
}

type DeleteInput struct {
    TraceId string
    Context string
    ID      string
    ActorID int // user dari token
}

type DeleteOutput struct {
    Success   bool
    Message   string
    ErrorCode string
}
```

> Semua output membawa `Success`/`Message`/`ErrorCode` (pola repo
> "typed operation results" di `about-this-project.md`). Konteks tak terdaftar
> atau salah konteks → `validation`; bukan pemilik → `forbidden`.

### 3.3 Alur per operasi

**Store**
1. Cek konteks terdaftar di registry (in-memory map, diisi `Register`).
2. Baca 512 byte pertama dari `File` → sniff via `mimetype.DetectReader`.
3. Sniffed MIME ∉ `MIMEAllow` → tolak (`validation`). Content-Type client diabaikan.
4. Bangun key: `<context>/<ownerID>/<xid><ext>` — `xid.New()`, `ext` dari `mimetype.Extension()`.
5. Stream sisa ke R2 via adapter `ObjectStore.Put` dengan counting reader — abort error bila byte > `MaxSize`; cleanup partial key bila gagal.
6. Insert baris `file_uploads` (`id` = xid yang sama dengan di key).

**DownloadURL**
1. `Storage.GetByID(id)` → tak ada → `not_found`.
2. `owner_id != viewer_id` → `forbidden`.
3. `ObjectStore.PresignedGet(key, expiry)` → return URL. Handler domain tinggal `307 redirect`.

**Delete**
1. `Storage.GetByID(id)` → tak ada → `not_found`.
2. `owner_id != actor_id` → `forbidden`.
3. `ObjectStore.Remove(key)` → `Storage.DeleteByID(id)` (urutan ini: object dihapus dulu supaya tidak ada file "yatim" yang tetap bisa dipresign).

### 3.4 Seam ObjectStore (kunci testability)

`minio.Client` dibungkus interface sempit:

```go
type ObjectStore interface {
    Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
    PresignedGet(ctx context.Context, key string, expiry time.Duration) (string, error)
    Remove(ctx context.Context, key string) error
}
```

- Implementasi produksi: adapter minio-go (`PutObject` dengan size `-1` untuk
  streaming chunked, `PresignedGetObject`, `RemoveObject`).
- Test: fake counterfeiter — service test tidak butuh R2 sungguhan
  (konsisten pola repo: mock hanya di cross-module seam).

---

## 4. Skema SQL

`src/app/upload/sql/20260101_000001_create_file_uploads.sql`:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS file_uploads (
    id            text PRIMARY KEY,      -- xid, dibangkitkan server-side (sama dgn <xid> di key)
    context       varchar NOT NULL,      -- konteks terdaftar, mis. 'avatar', 'receipt'
    owner_id      integer NOT NULL,      -- users.id (repo tidak pakai FK, ikut gaya itu)
    key           text NOT NULL UNIQUE,  -- <context>/<ownerID>/<xid><ext>, derivable dr kolom
    mime_type     varchar NOT NULL,      -- hasil sniff (Q2/Q6)
    size_bytes    bigint NOT NULL,
    original_name varchar(255) NOT NULL, -- utk Content-Disposition saat download
    created_at    timestamptz DEFAULT now(),
    updated_at    timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_file_uploads_owner ON file_uploads (owner_id, context);

-- +goose Down
DROP TABLE IF EXISTS file_uploads;
```

Registrasi domain di `database/migrator.go`:

```go
{Name: "upload", TableName: "goose_db_version_upload", FS: upload.Migrations}
```

(+ slice `UploadOnly` bila diperlukan; `make migrate-up upload` otomatis jalan
karena `cmd/migrate` memfilter lewat `database.All`.)

---

## 5. Config & Env Vars

Tambahan di `config/config.go`:

```go
// Cloudflare R2 (S3-compatible object storage)
R2AccountID     string
R2AccessKeyID   string
R2SecretAccessKey string
R2Bucket        string
// Endpoint R2 diturunkan: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", R2AccountID)
```

`.env.example`:

```env
# Cloudflare R2 (S3-compatible object storage)
R2_ACCOUNT_ID=
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_BUCKET=
```

- `R2_PUBLIC_URL` **tidak** dibutuhkan di MVP (semua konteks private, Q5).
- Region R2 selalu `"auto"` (di-hardcode di adapter, bukan env).

Wiring di `src/main.go` `buildApp` (foundation section, sebelum app modules):

```go
r2Client, err := minio.New(conf.R2Endpoint(), &minio.Options{
    Creds:  credentials.NewStaticV4(conf.R2AccessKeyID, conf.R2SecretAccessKey, ""),
    Secure: true,
})
uploadService := upload.NewService(upload.Config{
    Endpoint:        conf.R2Endpoint(),
    AccessKeyID:     conf.R2AccessKeyID,
    SecretAccessKey: conf.R2SecretAccessKey,
    Bucket:          conf.R2Bucket,
}, upload.NewStorage(db.Pool))
```

`App` struct: `UploadService upload.Service`.

---

## 6. Langkah Implementasi Berurutan

1. **Dependency** — `go get github.com/minio/minio-go/v7 github.com/gabriel-vasile/mimetype`; `go mod tidy`.
2. **Config** — tambah field `R2*` + helper `R2Endpoint()` di `config/config.go`; update `.env.example`; perluas `config_test.go`.
3. **Migration** — tulis `src/app/upload/sql/20260101_000001_create_file_uploads.sql` + `migrate.go`; daftarkan domain di `database/migrator.go`; verifikasi `make migrate-up upload` + `make migrate-status`.
4. **Storage layer** — `storage_interface.go` (`GetByID`, `Insert`, `DeleteByID` + kategorisasi error `ErrTypeNotFound` dll) + `storage.go` (pgx) + `storage_test.go` (suite real DB).
5. **ObjectStore seam** — `object_store.go` adapter minio-go + interface (3.4).
6. **Service contract** — `service_interface.go`: `Context`, `Config`, `Service`, named input/output (3.2).
7. **Service logic** — `service.go`:
   - `Register` (validasi + map in-memory).
   - `Store` (sniff 512B → policy → streaming Put → insert row; cleanup partial key on error).
   - `DownloadURL` (ownership → presign, default expiry 15 menit).
   - `Delete` (ownership → Remove → delete row).
8. **Wiring** — `src/main.go`: bangun minio client + `upload.NewService` di foundation section; `App.UploadService`.
9. **Tests** — `service_test.go` (suite: real DB untuk `file_uploads`, fake `ObjectStore` per skenario — validasi MIME reject, oversize abort, ownership forbidden, not-found, delete happy path); `setup_test.go` + `testing_helper_test.go`; generated fakes via counterfeiter (`gen_mock.go`).
10. **Verifikasi** — `gofmt`, `go vet ./...`, `go build ./...`, `go test ./... -count=1`.
11. *(Opsional, post-review)* — contoh integrasi satu domain pemakai (mis. route avatar di user domain: cek permission → panggil `upload.Store` → `307` ke `upload.DownloadURL`) sebagai bukti jalan; plus `BodyLimit` Skipper untuk path upload.

---

## 7. Catatan Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| **Partial/aborted object di R2** saat upload gagal (oversize / error stream) | Object yatim tanpa baris DB | Cleanup `Remove(key)` pada error path `Store`; log bila cleanup gagal |
| **Chunked upload (size = -1) ke R2** | Streaming tanpa Content-Length bisa ditolak/tidak didukung sebagian setup | Verifikasi saat implementasi; fallback spool ke temp file (dibatasi `MaxSize`) |
| **`BodyLimit` bypass salah sasaran** | Route non-upload ikut kehilangan proteksi 1M | `Skipper` harus match hanya path upload yang terdaftar; test handler memastikan route lain tetap kena limit |
| **Registry konteks in-memory** | Konteks tak ter-register saat startup → semua request 400/500 | Wajib `Register` di `buildApp`; error jelas saat `Store` menerima konteks tak dikenal; test wiring |
| **Clock skew untuk presigned URL** | URL expire lebih cepat/lambat dari 15 menit | Expiry 15 menit memberi margin; dokumentasikan batas skew |
| **Orphan row bila hapus R2 berhasil tapi hapus row gagal** | Row menggantung (presign → 404) | Log + retry manual; konvensi: hapus object dulu, row menyusul |
| **Sniff 512B tidak cukup untuk format tertentu** | Salah klasifikasi MIME | `gabriel-vasile/mimetype` sudah kuat; tambah kasus test untuk format yang didukung tiap konteks |
| **Tanpa FK `owner_id`** | User dihapus → baris file_uploads yatim | Konsisten gaya repo; cleanup user (jika ada) harus menyertakan upload module di masa depan |
| **minio-go vs aws-sdk trade-off** | Vendor lock minor ke minio-go | R2 S3-compatible; adapter tipis → ganti client mudah (seam 3.4) |
| **Uji terhadap R2 asli** | Test unit tidak membuktikan kompatibilitas endpoint | Sediakan integration test opsional (env `R2_*` tersedia) atau lokal MinIO (`MINIO_ROOT_USER/PASSWORD`) |
| **super_user lintas-owner** | Lib strict `viewer == owner` | Kesepakatan Q8: lib murni teknis — bypass super_user menjadi tanggung jawab domain pemanggil (cek permission lalu panggil dengan ownerID yang benar) |

---

## 8. Ringkasan Singkat

**Grilling Q1–Q7 selesai — seluruh paket MVP disetujui ('Ok').**

- **Q1** Upload generic per konteks · **Q2** Support lib configurable by caller, validasi WAJIB MIME (sniff) · **Q3** Satu bucket + prefix, key `<context>/<ownerID>/<xid><ext>` · **Q4** Tabel penuh `file_uploads`, `id`=xid, `original_name` disimpan · **Q5** Upload streaming (bypass BodyLimit, enforce di service) + download presigned URL, semua konteks private · **Q6** Sniffing `gabriel-vasile/mimetype`, lib murni teknis (permission di domain pemanggil) · **Q7** Struktur `src/app/upload/`, API `Register/Store/DownloadURL/Delete`, expiry presign 15 menit, Delete in MVP, tanpa listing/quota/public/rename.

- **Folded:** env `R2_ACCOUNT_ID/_ACCESS_KEY_ID/_SECRET_ACCESS_KEY/_BUCKET` (endpoint derivable, tanpa `R2_PUBLIC_URL`); Go client **minio-go/v7** di balik seam `ObjectStore`.

- **Deferred:** rate limiting per-konteks, public flag, listing, quota, rename, virus-scan.

- **Implementasi (10 langkah):** deps → config → migration `file_uploads` (+ register di migrator) → storage → ObjectStore adapter → kontrak service → logika service → wiring `main.go` → tests (suite real DB + fake ObjectStore) → verifikasi `build`/`test` → (opsional) contoh integrasi domain + BodyLimit Skipper.

- **Risiko utama:** partial object on failed upload (cleanup), chunked upload R2 (verifikasi), BodyLimit skipper yang tepat sasaran, registry konteks wajib ter-register di startup.

**Next:** review user → implementasi per langkah 1–10 di atas.
