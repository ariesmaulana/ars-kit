package user

import (
	"context"

	"github.com/ariesmaulana/ars-kit/src/app/workflow"
)

// avatarKeyUpdater adapts Storage to the workflow package's AvatarKeyUpdater
// seam (BeginTx → UpdateAvatarKey → Commit) so the avatar_upload workflow
// step can persist the new key without the workflow package importing user.
type avatarKeyUpdater struct {
	storage Storage
}

// NewAvatarKeyUpdater builds the workflow AvatarKeyUpdater over the user
// storage. Wire it once in main alongside the avatar uploader.
func NewAvatarKeyUpdater(storage Storage) workflow.AvatarKeyUpdater {
	return &avatarKeyUpdater{storage: storage}
}

func (a *avatarKeyUpdater) UpdateAvatarKey(ctx context.Context, userID int, avatarKey string) error {
	db, err := a.storage.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer db.Rollback()
	if err := db.UpdateAvatarKey(ctx, userID, avatarKey); err != nil {
		return err
	}
	return db.Commit()
}
