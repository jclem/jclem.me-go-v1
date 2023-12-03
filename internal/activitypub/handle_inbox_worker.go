package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/riverqueue/river"
)

type HandleInboxArgs struct {
	// ActivityID is the *object* ID of the activity.
	ActivityID string `json:"activity_id"`

	// UserRecordID is the ID of the user that the activity is for.
	UserRecordID int64 `json:"user_record_id"`
}

func (a HandleInboxArgs) Kind() string {
	return "handle-inbox"
}

type HandleInboxWorker struct {
	river.WorkerDefaults[HandleInboxArgs]
	pub *Service
	id  *identity.Service
}

func (w *HandleInboxWorker) Work(ctx context.Context, job *river.Job[HandleInboxArgs]) error {
	ar, err := w.pub.GetActivityByID(ctx, job.Args.UserRecordID, job.Args.ActivityID)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	var ao Activity[any]
	if err := json.Unmarshal(ar.Data, &ao); err != nil {
		return fmt.Errorf("failed to unmarshal activity data: %w", err)
	}

	switch ao.Type {
	case followActivityType:
		return w.handleFollow(ctx, job.Args.UserRecordID, ar, ao)
	case undoActivityType:
		return w.handleUndo(ctx, job.Args.UserRecordID, ar, ao)
	}

	return nil
}

func (w *HandleInboxWorker) handleFollow(ctx context.Context, userRecordID int64, ar ActivityRecord, ao Activity[any]) error {
	if err := w.createFollower(ctx, userRecordID, ar, ao.Actor); err != nil {
		slog.ErrorContext(ctx, "failed to create follower", "error", err)

		return err
	}

	if err := w.acceptActivity(ctx, userRecordID, ar, ao.Actor); err != nil {
		slog.ErrorContext(ctx, "failed to accept follower", "error", err)

		return err
	}

	return nil
}

func (w *HandleInboxWorker) handleUndo(ctx context.Context, userRecordID int64, ar ActivityRecord, ao Activity[any]) error {
	// Serialize and deserialize the activity's object to get an Activity[string] struct (the follow).
	j, err := json.Marshal(ao.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	var fa Activity[any]
	if err := json.Unmarshal(j, &fa); err != nil {
		return fmt.Errorf("failed to unmarshal follow activity: %w", err)
	}

	// Ensure the undo actor and the activity actor are the same.
	if ao.Actor != fa.Actor {
		return fmt.Errorf("actor and undo actor are not the same")
	}

	if fa.Type != followActivityType {
		return fmt.Errorf("activity is not a follow")
	}

	if err := w.pub.DeleteFollower(ctx, userRecordID, fa.Actor); err != nil {
		return fmt.Errorf("failed to delete follower: %w", err)
	}

	if err := w.acceptActivity(ctx, userRecordID, ar, ao.Actor); err != nil {
		return fmt.Errorf("failed to accept undo: %w", err)
	}

	return nil
}

func (w *HandleInboxWorker) createFollower(ctx context.Context, userRecordID int64, activity ActivityRecord, actorID string) error {
	if activity.Type != followActivityType {
		return fmt.Errorf("activity is not a follow")
	}

	follower, err := w.pub.CreateFollower(ctx, userRecordID, actorID, activity.ID)
	if err != nil {
		return fmt.Errorf("failed to create follower: %w", err)
	}

	slog.InfoContext(ctx, "created follower", "id", follower.ActorID)

	return nil
}

func (w *HandleInboxWorker) acceptActivity(ctx context.Context, userRecordID int64, activity ActivityRecord, actorID string) error {
	user, err := w.id.GetUserByID(ctx, userRecordID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	actor, err := GetActor(ctx, actorID)
	if err != nil {
		return fmt.Errorf("error getting actor: %w", err)
	}

	inboxURL := actor.Inbox
	if inboxURL == "" {
		return errors.New("actor has no inbox")
	}

	accept := newAcceptActivity(ActorID(user), activity.ID)

	j, err := json.Marshal(accept)
	if err != nil {
		return fmt.Errorf("failed to marshal accept: %w", err)
	}

	req, err := newSignedActivityRequest(ctx, w.id, userRecordID, http.MethodPost, inboxURL, j)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error posting accept: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "error closing accept response body", "error", err)
		}
	}()

	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		return fmt.Errorf("error posting accept: %s", resp.Status)
	}

	return nil
}

func newHandleFollowWorker(pub *Service, id *identity.Service) *HandleInboxWorker {
	return &HandleInboxWorker{
		id:  id,
		pub: pub,
	}
}
