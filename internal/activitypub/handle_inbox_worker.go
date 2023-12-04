package activitypub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/jclem/jclem.me/internal/database"
	"github.com/riverqueue/river"
)

type HandleInboxArgs struct {
	// ActivityID is the *object* ID of the activity.
	ActivityID string `json:"activity_id"`

	// UserRecordID is the ID of the user that the activity is for.
	UserRecordID database.ULID `json:"user_record_id"`
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
		err = fmt.Errorf("failed to get activity: %w", err)
		if errors.Is(err, ErrActivityNotFound) {
			return river.JobCancel(err) //nolint:wrapcheck
		}

		return err
	}

	var ao Activity[any]
	if err := json.Unmarshal(ar.Data, &ao); err != nil {
		return river.JobCancel(fmt.Errorf("failed to unmarshal activity data: %w", err)) //nolint:wrapcheck
	}

	switch ao.Type {
	case followActivityType:
		return w.handleFollow(ctx, job.Args.UserRecordID, ar, ao)
	case undoActivityType:
		return w.handleUndo(ctx, job.Args.UserRecordID, ar, ao)
	}

	return nil
}

func (w *HandleInboxWorker) handleFollow(ctx context.Context, userRecordID database.ULID, ar ActivityRecord, ao Activity[any]) error {
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

func (w *HandleInboxWorker) handleUndo(ctx context.Context, userRecordID database.ULID, ar ActivityRecord, ao Activity[any]) error {
	// Serialize and deserialize the activity's object to get an Activity[string] struct (the follow).
	j, err := json.Marshal(ao.Object)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	var undoneActivity Activity[any]
	if err := json.Unmarshal(j, &undoneActivity); err != nil {
		return river.JobCancel(fmt.Errorf("failed to unmarshal object: %w", err)) //nolint:wrapcheck
	}

	// Ensure the undo actor and the activity actor are the same.
	if ao.Actor != undoneActivity.Actor {
		return river.JobCancel(fmt.Errorf("actor and undo actor are not the same: %s != %s", ao.Actor, undoneActivity.Actor)) //nolint:wrapcheck
	}

	if undoneActivity.Type != followActivityType {
		return river.JobCancel(fmt.Errorf("activity is not a follow: %s", undoneActivity.Type)) //nolint:wrapcheck
	}

	if err := w.pub.DeleteFollower(ctx, userRecordID, undoneActivity.Actor); err != nil {
		return fmt.Errorf("failed to delete follower: %w", err)
	}

	if err := w.acceptActivity(ctx, userRecordID, ar, ao.Actor); err != nil {
		return fmt.Errorf("failed to accept undo: %w", err)
	}

	return nil
}

func (w *HandleInboxWorker) createFollower(ctx context.Context, userRecordID database.ULID, activity ActivityRecord, actorID string) error {
	if activity.Type != followActivityType {
		return river.JobCancel(fmt.Errorf("activity is not a follow: %s", activity.Type)) //nolint:wrapcheck
	}

	_, err := w.pub.CreateFollower(ctx, userRecordID, actorID, activity.ID)
	if err != nil {
		return fmt.Errorf("failed to create follower: %w", err)
	}

	return nil
}

func (w *HandleInboxWorker) acceptActivity(ctx context.Context, userRecordID database.ULID, activity ActivityRecord, actorID string) error {
	user, err := w.id.GetUserByID(ctx, userRecordID)
	if err != nil {
		err = fmt.Errorf("failed to get user: %w", err)
		if errors.Is(err, identity.ErrUserNotFound) {
			return river.JobCancel(err) //nolint:wrapcheck
		}

		return err
	}

	actor, err := GetActor(ctx, actorID)
	if err != nil {
		return fmt.Errorf("error getting actor: %w", err)
	}

	inboxURL := actor.Inbox
	if inboxURL == "" {
		return river.JobCancel(fmt.Errorf("actor has no inbox: %s", actor.ID)) //nolint:wrapcheck
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
		if resp.StatusCode >= 500 {
			return fmt.Errorf("error posting accept: %s", resp.Status)
		}

		return river.JobCancel(fmt.Errorf("error posting accept: %s", resp.Status)) //nolint:wrapcheck
	}

	return nil
}

func newHandleFollowWorker(pub *Service, id *identity.Service) *HandleInboxWorker {
	return &HandleInboxWorker{
		id:  id,
		pub: pub,
	}
}
