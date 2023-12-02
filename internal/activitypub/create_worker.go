package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

type HandleCreateArgs struct {
	// ActivityID is the *object* ID of the activity.
	ActivityID string `json:"activity_id"`
}

func (a HandleCreateArgs) Kind() string {
	return "handle-create"
}

type HandleCreateWorker struct {
	river.WorkerDefaults[HandleCreateArgs]
	pub *Service
}

func (w *HandleCreateWorker) Work(ctx context.Context, job *river.Job[HandleCreateArgs]) error {
	activity, err := w.pub.GetActivityByID(ctx, job.Args.ActivityID)
	if err != nil {
		return fmt.Errorf("failed to get activity: %w", err)
	}

	var a Activity[Note]
	if err := json.Unmarshal(activity.Data, &a); err != nil {
		slog.Error("failed to unmarshal activity data", "error", err)

		return fmt.Errorf("failed to unmarshal activity data: %w", err)
	}

	note, err := w.pub.CreateNote(ctx, a.ID, a.Object)
	if err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}

	slog.InfoContext(ctx, "created note", "id", note.RecordID)

	return nil
}

func newHandleCreateWorker(pub *Service) *HandleCreateWorker {
	return &HandleCreateWorker{
		pub: pub,
	}
}
