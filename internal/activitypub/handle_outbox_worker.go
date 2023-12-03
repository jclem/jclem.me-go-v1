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

type HandleOutboxArgs struct {
	// ActivityID is the object ID of the activity.
	ActivityID string `json:"activity_id"`

	// FollowerID is the object ID of the follower that the activity is for.
	FollowerID string `json:"follower_id"`

	// UserRecordID is the ID of the user that the activity is for.
	UserRecordID int64 `json:"user_record_id"`
}

func (a HandleOutboxArgs) Kind() string {
	return "handle-outbox"
}

type HandleOutboxWorker struct {
	river.WorkerDefaults[HandleOutboxArgs]
	id  *identity.Service
	pub *Service
}

// Work implements the river.Worker interface.
//
// It functions by fetching newly-created activity and delivering it to the
// inbox of the follower denoted in the job.
func (w *HandleOutboxWorker) Work(ctx context.Context, job *river.Job[HandleOutboxArgs]) error { //nolint:cyclop
	activity, err := w.pub.GetActivityByID(ctx, job.Args.UserRecordID, job.Args.ActivityID)
	if err != nil {
		if errors.Is(err, ErrActivityNotFound) {
			return river.JobCancel(fmt.Errorf("activity not found: %s", job.Args.ActivityID)) //nolint:wrapcheck
		}

		return fmt.Errorf("failed to get activity: %w", err)
	}

	var a Activity[any]
	if err := json.Unmarshal(activity.Data, &a); err != nil {
		return river.JobCancel(fmt.Errorf("failed to unmarshal activity data: %w", err)) //nolint:wrapcheck
	}

	actor, err := GetActor(ctx, job.Args.FollowerID)
	if err != nil {
		return fmt.Errorf("failed to get actor: %w", err)
	}

	if actor.Inbox == "" {
		return river.JobCancel(fmt.Errorf("follower has no inbox: %s", actor.ID)) //nolint:wrapcheck
	}

	j, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	req, err := newSignedActivityRequest(ctx, w.id, job.Args.UserRecordID, http.MethodPost, actor.Inbox, j)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close response body", "error", err)
		}
	}()

	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		return fmt.Errorf("error posting activity: %s", resp.Status)
	}

	return nil
}

func newHandleOutboxWorker(pub *Service, id *identity.Service) *HandleOutboxWorker {
	return &HandleOutboxWorker{
		id:  id,
		pub: pub,
	}
}
