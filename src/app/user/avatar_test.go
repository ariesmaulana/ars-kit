package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/upload"
	"github.com/ariesmaulana/ars-kit/src/app/upload/uploadfakes"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/app/workflow"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────
// Storage layer
// ──────────────────────────────────────────────────────────────

func TestStorageUpdateAvatarKey(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Storage UpdateAvatarKey", func() {
			suite.Runs(t, "Should persist avatar_key and return it in reads", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avataruser", "avatar@example.com", "Avatar User", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)
				defer tx.Rollback()

				err = tx.UpdateAvatarKey(ctx, int(u.Id), "avatars/xid.jpg")
				assert.Nil(t, err)

				err = tx.Commit()
				assert.Nil(t, err)

				updated := app.Helper.GetUserById(ctx, t, int(u.Id))
				assert.NotNil(t, updated.AvatarKey)
				assert.Equal(t, "avatars/xid.jpg", *updated.AvatarKey)
			})

			suite.Runs(t, "Should rollback avatar_key change when transaction is not committed", func(t *testing.T, appCtx *testsuite.AppContext) {
				app := initUserApp(appCtx)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avataruser2", "avatar2@example.com", "Avatar User 2", "password123")

				tx, err := app.Storage.BeginTx(ctx)
				assert.Nil(t, err)

				err = tx.UpdateAvatarKey(ctx, int(u.Id), "avatars/rollback.jpg")
				assert.Nil(t, err)
				tx.Rollback()

				updated := app.Helper.GetUserById(ctx, t, int(u.Id))
				assert.Nil(t, updated.AvatarKey)
			})
		})
	})
}

// ──────────────────────────────────────────────────────────────
// Service layer
// ──────────────────────────────────────────────────────────────

// avatarTestEngine registers the avatar_upload definition on a fresh
// default engine so UploadAvatar's workflow.Register succeeds in tests.
func avatarTestEngine(app *UserApp, uploader *uploadfakes.UploaderFake) (*workflow.Engine, workflow.Store, func()) {
	store := workflow.NewStore(app.Pool)
	engine := workflow.NewEngine(store, workflow.Config{})
	engine.Register(workflow.AvatarUploadWorkflow(uploader, user.NewAvatarKeyUpdater(app.Storage)))
	workflow.SetDefault(engine)
	return engine, store, func() { workflow.SetDefault(nil) }
}

// getAvatarJob reads a workflow_job row back into an Entity the Executor
// can run. The Store intentionally has no read-by-id method.
func getAvatarJob(ctx context.Context, t *testing.T, app *UserApp, traceID string) *workflow.Entity {
	t.Helper()
	var e workflow.Entity
	var payload []byte
	var status string
	err := app.Pool.QueryRow(ctx, `SELECT id, workflow_name, trace_id, payload, status, current_step, retry_count FROM workflow_job WHERE trace_id = $1 AND workflow_name = $2`, traceID, "avatar_upload").Scan(
		&e.ID, &e.WorkflowName, &e.TraceID, &payload, &status, &e.CurrentStep, &e.RetryCount)
	require.NoError(t, err)
	e.Payload = payload
	e.Status = workflow.Status(status)
	return &e
}

func TestServiceUploadAvatar(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Runs(t, "Service UploadAvatar", func(t *testing.T, appCtx *testsuite.AppContext) {
			t.Run("Should stage file and enqueue avatar_upload job", func(t *testing.T) {
				stagingDir := t.TempDir()
				app := initUserAppWithStagingDir(appCtx, stagingDir)
				fakeUploader := &uploadfakes.UploaderFake{}
				_, _, teardown := avatarTestEngine(app, fakeUploader)
				defer teardown()
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatarsvc", "avatarsvc@example.com", "Avatar Svc", "password123")
				body := []byte{0xff, 0xd8, 0xff, 0xd9}

				output := app.Service.UploadAvatar(ctx, &user.UploadAvatarInput{
					TraceId:  "trace-avatar",
					Id:       int(u.Id),
					Reader:   bytes.NewReader(body),
					Filename: "photo.jpg",
					SizeHint: 4,
				})

				require.True(t, output.Success, "expected success, got %+v", output)
				assert.Equal(t, "Avatar upload accepted", output.Message)
				assert.Equal(t, user.ErrorCodeNone, output.ErrorCode)

				// The request path never touches the object store.
				assert.Equal(t, 0, fakeUploader.UploadCallCount())

				// One job queued for this trace.
				assert.Equal(t, 1, app.Helper.CountWorkflowJobs(ctx, t, "trace-avatar", "avatar_upload"))

				// Payload carries the minted key, the staged file, and no old key.
				var raw []byte
				err := app.Pool.QueryRow(ctx, `SELECT payload FROM workflow_job WHERE trace_id = $1 AND workflow_name = $2`, "trace-avatar", "avatar_upload").Scan(&raw)
				require.NoError(t, err)
				var payload workflow.AvatarUploadPayload
				require.NoError(t, json.Unmarshal(raw, &payload))
				assert.Equal(t, int(u.Id), payload.UserID)
				assert.True(t, strings.HasSuffix(payload.Key, ".jpg"), "key %q keeps the filename extension", payload.Key)
				assert.Equal(t, "", payload.OldKey)
				assert.Equal(t, "photo.jpg", payload.Filename)

				// The staged file exists on disk with the request bytes.
				staged, err := os.ReadFile(payload.StagedPath)
				require.NoError(t, err)
				assert.Equal(t, body, staged)
				assert.Equal(t, filepath.Join(stagingDir, "trace-avatar"), filepath.Dir(payload.StagedPath))

				// avatar_key is untouched until the worker runs.
				updated := app.Helper.GetUserById(ctx, t, int(u.Id))
				assert.Nil(t, updated.AvatarKey)
			})

			t.Run("Should pass the previous avatar key to the job", func(t *testing.T) {
				stagingDir := t.TempDir()
				app := initUserAppWithStagingDir(appCtx, stagingDir)
				fakeUploader := &uploadfakes.UploaderFake{}
				_, _, teardown := avatarTestEngine(app, fakeUploader)
				defer teardown()
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatarold", "avatarold@example.com", "Avatar Old", "password123")
				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				require.NoError(t, tx.UpdateAvatarKey(ctx, int(u.Id), "avatars/old.jpg"))
				require.NoError(t, tx.Commit())

				output := app.Service.UploadAvatar(ctx, &user.UploadAvatarInput{
					TraceId:  "trace-avatar-old",
					Id:       int(u.Id),
					Reader:   bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xd9}),
					Filename: "photo.jpg",
					SizeHint: 4,
				})
				require.True(t, output.Success, "expected success, got %+v", output)

				var raw []byte
				err = app.Pool.QueryRow(ctx, `SELECT payload FROM workflow_job WHERE trace_id = $1 AND workflow_name = $2`, "trace-avatar-old", "avatar_upload").Scan(&raw)
				require.NoError(t, err)
				var payload workflow.AvatarUploadPayload
				require.NoError(t, json.Unmarshal(raw, &payload))
				assert.Equal(t, "avatars/old.jpg", payload.OldKey)
			})

			t.Run("Should reject invalid input without staging or enqueue", func(t *testing.T) {
				stagingDir := t.TempDir()
				app := initUserAppWithStagingDir(appCtx, stagingDir)
				fakeUploader := &uploadfakes.UploaderFake{}
				_, _, teardown := avatarTestEngine(app, fakeUploader)
				defer teardown()
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatarrej", "avatarrej@example.com", "Avatar Rej", "password123")
				oversize := bytes.Repeat([]byte{0x01}, 2*1024*1024+1)

				rows := []struct {
					name    string
					input   *user.UploadAvatarInput
					traceID string
					message string
					code    user.ErrorCode
				}{
					{"empty trace id", &user.UploadAvatarInput{TraceId: "", Id: int(u.Id), Reader: bytes.NewReader([]byte{0x01}), Filename: "a.jpg"}, "trace-rej-empty", "TraceId is mandatory", user.ErrorCodeValidation},
					{"empty user id", &user.UploadAvatarInput{TraceId: "trace-rej-noid", Id: 0, Reader: bytes.NewReader([]byte{0x01}), Filename: "a.jpg"}, "trace-rej-noid", "User ID is mandatory", user.ErrorCodeValidation},
					{"nil reader", &user.UploadAvatarInput{TraceId: "trace-rej-noreader", Id: int(u.Id), Reader: nil, Filename: "a.jpg"}, "trace-rej-noreader", "avatar file is required", user.ErrorCodeValidation},
					{"empty filename", &user.UploadAvatarInput{TraceId: "trace-rej-nofname", Id: int(u.Id), Reader: bytes.NewReader([]byte{0x01}), Filename: "  "}, "trace-rej-nofname", "avatar filename is required", user.ErrorCodeValidation},
					{"unknown user", &user.UploadAvatarInput{TraceId: "trace-rej-nouser", Id: 999999999, Reader: bytes.NewReader([]byte{0x01}), Filename: "a.jpg"}, "trace-rej-nouser", "User not found", user.ErrorCodeValidation},
					{"oversize file", &user.UploadAvatarInput{TraceId: "trace-rej-big", Id: int(u.Id), Reader: bytes.NewReader(oversize), Filename: "a.jpg", SizeHint: int64(len(oversize))}, "trace-rej-big", "avatar must be ≤ 2MB", user.ErrorCodeValidation},
				}

				for _, row := range rows {
					t.Run(row.name, func(t *testing.T) {
						output := app.Service.UploadAvatar(ctx, row.input)
						assert.False(t, output.Success)
						assert.Equal(t, row.code, output.ErrorCode)
						assert.Equal(t, row.message, output.Message)
						assert.Equal(t, 0, app.Helper.CountWorkflowJobs(ctx, t, row.traceID, "avatar_upload"))
					})
				}

				// Nothing staged for any rejected row.
				entries, err := os.ReadDir(stagingDir)
				require.NoError(t, err)
				assert.Empty(t, entries)
				assert.Equal(t, 0, fakeUploader.UploadCallCount())
			})

			t.Run("Should run UploadFile then Cleanup end to end", func(t *testing.T) {
				stagingDir := t.TempDir()
				app := initUserAppWithStagingDir(appCtx, stagingDir)
				fakeUploader := &uploadfakes.UploaderFake{}
				// Echo the service-minted key like the real lib does with KeyOverride.
				fakeUploader.UploadStub = func(_ context.Context, req upload.UploadRequest) (*upload.UploadResult, error) {
					return &upload.UploadResult{Key: req.KeyOverride, MIME: "image/jpeg", Size: req.SizeHint, Extension: ".jpg"}, nil
				}
				engine, store, teardown := avatarTestEngine(app, fakeUploader)
				defer teardown()
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatare2e", "avatare2e@example.com", "Avatar E2E", "password123")
				tx, err := app.Storage.BeginTx(ctx)
				require.NoError(t, err)
				require.NoError(t, tx.UpdateAvatarKey(ctx, int(u.Id), "avatars/old.jpg"))
				require.NoError(t, tx.Commit())

				output := app.Service.UploadAvatar(ctx, &user.UploadAvatarInput{
					TraceId:  "trace-avatar-e2e",
					Id:       int(u.Id),
					Reader:   bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xd9}),
					Filename: "photo.jpg",
					SizeHint: 4,
				})
				require.True(t, output.Success, "expected success, got %+v", output)

				executor := workflow.NewExecutor(engine, store)

				// Step 1: upload + persist avatar_key.
				step1 := getAvatarJob(ctx, t, app, "trace-avatar-e2e")
				require.Equal(t, "UploadFile", step1.CurrentStep)
				require.NoError(t, executor.Execute(ctx, step1))
				require.Equal(t, 1, fakeUploader.UploadCallCount())
				_, req := fakeUploader.UploadArgsForCall(0)
				assert.Equal(t, "photo.jpg", req.Filename)

				step2 := getAvatarJob(ctx, t, app, "trace-avatar-e2e")
				assert.Equal(t, "Cleanup", step2.CurrentStep)
				var payload workflow.AvatarUploadPayload
				require.NoError(t, json.Unmarshal(step2.Payload, &payload))
				assert.Equal(t, "image/jpeg", payload.MIME)

				updated := app.Helper.GetUserById(ctx, t, int(u.Id))
				require.NotNil(t, updated.AvatarKey)
				assert.Equal(t, payload.Key, *updated.AvatarKey)

				// Step 2: old avatar deleted, staged file removed.
				require.NoError(t, executor.Execute(ctx, step2))
				require.Equal(t, 1, fakeUploader.DeleteCallCount())
				_, oldKey := fakeUploader.DeleteArgsForCall(0)
				assert.Equal(t, "avatars/old.jpg", oldKey)
				_, statErr := os.Stat(payload.StagedPath)
				assert.True(t, os.IsNotExist(statErr), "staged file should be removed")

				done := getAvatarJob(ctx, t, app, "trace-avatar-e2e")
				assert.Equal(t, workflow.StatusDone, done.Status)
			})
		})
	})
}
