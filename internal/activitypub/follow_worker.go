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

type HandleFollowArgs struct {
	// ActivityID is the *object* ID of the activity.
	ActivityID string `json:"activity_id"`

	// UserRecordID is the ID of the user that the activity is for.
	UserRecordID int64 `json:"user_record_id"`
}

func (a HandleFollowArgs) Kind() string {
	return "handle-follow"
}

type HandleFollowWorker struct {
	river.WorkerDefaults[HandleFollowArgs]
	pub *Service
	id  *identity.Service
}

func (w *HandleFollowWorker) Work(ctx context.Context, job *river.Job[HandleFollowArgs]) error {
	activity, err := w.pub.GetActivityByID(ctx, job.Args.UserRecordID, job.Args.ActivityID)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	var data map[string]any
	if err := json.Unmarshal(activity.Data, &data); err != nil {
		return fmt.Errorf("failed to unmarshal activity data: %w", err)
	}

	var actorID string

	actor := data["actor"]

	switch t := actor.(type) {
	case string:
		actorID = t
	case map[string]any:
		id, ok := t["id"].(string)
		if !ok {
			return fmt.Errorf("failed to get actor ID")
		}

		actorID = id
	default:
		return fmt.Errorf("unexpected actor type: %T", t)
	}

	if err := w.createFollower(ctx, job.Args.UserRecordID, activity, actorID); err != nil {
		slog.ErrorContext(ctx, "failed to create follower", "error", err)

		return err
	}

	if err := w.acceptFollower(ctx, job.Args.UserRecordID, activity, actorID); err != nil {
		slog.ErrorContext(ctx, "failed to accept follower", "error", err)

		return err
	}

	return nil
}

func (w *HandleFollowWorker) createFollower(ctx context.Context, userRecordID int64, activity ActivityRecord, actorID string) error {
	if activity.Type != "Follow" {
		return fmt.Errorf("activity is not a follow")
	}

	follower, err := w.pub.CreateFollower(ctx, userRecordID, actorID, activity.ID)
	if err != nil {
		return fmt.Errorf("failed to create follower: %w", err)
	}

	slog.InfoContext(ctx, "created follower", "id", follower.ActorID)

	return nil
}

func (w *HandleFollowWorker) acceptFollower(ctx context.Context, userRecordID int64, activity ActivityRecord, actorID string) error {
	user, err := w.id.GetUserByID(ctx, userRecordID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	followerActor, err := GetActor(ctx, actorID)
	if err != nil {
		return fmt.Errorf("error getting actor: %w", err)
	}

	inboxURL := followerActor.Inbox
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

func newHandleFollowWorker(pub *Service, id *identity.Service) *HandleFollowWorker {
	return &HandleFollowWorker{
		id:  id,
		pub: pub,
	}
}
