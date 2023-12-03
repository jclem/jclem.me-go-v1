// Package activitypub provides ActivityPub client and server support.
package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ContentType is the content type for ActivityPub requests and responses.
const ContentType = "application/activity+json; charset=utf-8"

// Domain is the domain of the server.
const Domain = "pub.jclem.me"

// GetActor requests an actor by their ID.
func GetActor(ctx context.Context, actorID string) (Actor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorID, nil)
	if err != nil {
		return Actor{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", ContentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Actor{}, fmt.Errorf("failed to perform request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Actor{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var actor Actor
	if err := json.NewDecoder(resp.Body).Decode(&actor); err != nil {
		return Actor{}, fmt.Errorf("failed to decode actor: %w", err)
	}

	return actor, nil
}
