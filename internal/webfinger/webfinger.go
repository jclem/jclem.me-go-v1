// Package webfinger provides simple WebFinger client support.
package webfinger

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
