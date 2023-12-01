// Package webfinger provides simple WebFinger client support.
package webfinger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// A JRD is a JSON Resource Descriptor.
//
// SEE https://datatracker.ietf.org/doc/html/rfc7033#section-4.4
type JRD struct {
	Subject    string         `json:"subject,omitempty"`
	Aliases    []string       `json:"aliases,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Links      []Link         `json:"links,omitempty"`
}

// A Link is a JRD link.
//
// SEE https://datatracker.ietf.org/doc/html/rfc7033#section-4.4.4
type Link struct {
	Rel        string            `json:"rel,omitempty"`
	Type       string            `json:"type,omitempty"`
	Href       string            `json:"href,omitempty"`
	Titles     map[string]string `json:"titles,omitempty"`
	Properties map[string]any    `json:"properties,omitempty"`
}

// Path is the universal WebFinger path.
const Path = "/.well-known/webfinger"

// ContentType is the WebFinger content type.
const ContentType = "application/jrd+json"

// Request performs a WebFinger request.
//
// SEE https://datatracker.ietf.org/doc/html/rfc7033#section-4
func Request(ctx context.Context, domain string, resource string) (JRD, error) {
	url := fmt.Sprintf("https://%s%s?resource=%s", domain, Path, url.QueryEscape(resource))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return JRD{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", ContentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return JRD{}, fmt.Errorf("failed to perform request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return JRD{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var jrd JRD

	if err := json.NewDecoder(resp.Body).Decode(&jrd); err != nil {
		return JRD{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return jrd, nil
}
