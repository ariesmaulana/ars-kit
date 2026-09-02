package user_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/upload"
	"github.com/ariesmaulana/ars-kit/src/app/upload/uploadfakes"
	"github.com/ariesmaulana/ars-kit/src/app/user"
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

func TestServiceUploadAvatar(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Service UploadAvatar", func() {
			suite.Runs(t, "Should upload, persist key and return updated user", func(t *testing.T, appCtx *testsuite.AppContext) {
				fakeUploader := &uploadfakes.UploaderFake{}
				fakeUploader.UploadReturns(&upload.UploadResult{
					Key:       "avatars/xid.jpg",
					MIME:      "image/jpeg",
					Size:      1234,
					Extension: ".jpg",
				}, nil)

				app := initUserAppWithUploader(appCtx, fakeUploader)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatarsvc", "avatarsvc@example.com", "Avatar Svc", "password123")

				output := app.Service.UploadAvatar(ctx, &user.UploadAvatarInput{
					TraceId:  "trace-avatar",
					Id:       int(u.Id),
					Reader:   bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xd9}),
					Filename: "photo.jpg",
					SizeHint: 4,
				})

				require.True(t, output.Success, "expected success, got %+v", output)
				assert.Equal(t, "Avatar updated successfully", output.Message)
				assert.Equal(t, "avatars/xid.jpg", output.Key)
				assert.Equal(t, user.ErrorCodeNone, output.ErrorCode)

				// Uploader got the primitives, handler did not smuggle multipart artifacts
				require.Equal(t, 1, fakeUploader.UploadCallCount())
				_, req := fakeUploader.UploadArgsForCall(0)
				assert.Equal(t, "photo.jpg", req.Filename)
				assert.Equal(t, int64(4), req.SizeHint)

				// DB persisted the key
				updated := app.Helper.GetUserById(ctx, t, int(u.Id))
				assert.NotNil(t, updated.AvatarKey)
				assert.Equal(t, "avatars/xid.jpg", *updated.AvatarKey)
			})

			suite.Runs(t, "Should reject unsupported image type", func(t *testing.T, appCtx *testsuite.AppContext) {
				fakeUploader := &uploadfakes.UploaderFake{}
				fakeUploader.UploadReturns(nil, &upload.UploadError{
					Code: upload.ErrInvalidMIME,
					Op:   "upload",
					Err:  upload.ErrInvalidMIME,
				})

				app := initUserAppWithUploader(appCtx, fakeUploader)
				ctx := context.Background()

				u := app.Helper.InsertUser(ctx, t, "avatarbad", "avatarbad@example.com", "Avatar Bad", "password123")

				output := app.Service.UploadAvatar(ctx, &user.UploadAvatarInput{
					TraceId:  "trace-avatar",
					Id:       int(u.Id),
					Reader:   bytes.NewReader([]byte("not-an-image")),
					Filename: "photo.txt",
					SizeHint: 12,
				})

				assert.False(t, output.Success)
				assert.Equal(t, user.ErrorCodeValidation, output.ErrorCode)
				assert.Contains(t, output.Message, "jpeg/png/webp")
			})
		})
	})
}
