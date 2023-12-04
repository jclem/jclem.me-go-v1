package activitypub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jclem/jclem.me/internal/activitypub/identity"
	"github.com/jclem/jclem.me/internal/activitypub/orderedmap"
)

// ActivityStreamsContext is the ActivityStreams context.
const ActivityStreamsContext = "https://www.w3.org/ns/activitystreams"

// SecurityContext is the security context (for public keys on actors).
const SecurityContext = "https://w3id.org/security/v1"

// MastodonContext is the Mastodon context.
var MastodonContext = map[string]string{ //nolint:gochecknoglobals
	"toot":      "http://joinmastodon.org/ns#",
	"Hashtag":   "as:Hashtag",
	"sensitive": "as:sensitive",
}

// ActorContext is the context for actors.
//
// SEE https://docs.joinmastodon.org/spec/activitypub/#PropertyValue
// > Mastodon currently incorrectly expects and maps the term schema to the base
// > URI http://schema.org# instead of to the base URI https://schema.org/.
// > Therefore, JSON-LD processors who use the correct context definition will
// > fail to process profile fields correctly.
var ActorContext = NewContext(ActivityStreamsContext, SecurityContext, MastodonContext, map[string]string{ //nolint:gochecknoglobals
	"discoverable":              "toot:discoverable",
	"manuallyApprovesFollowers": "as:manuallyApprovesFollowers",
	"schema":                    "http://schema.org/#",
	"PropertyValue":             "schema:PropertyValue",
	"value":                     "schema:value",
})

// A Context is a JSON-LD context.
//
// Although there is a normative object definition for context at
// https://www.w3.org/TR/json-ld/#context-definitions, we use a simple any
// array, as we usually do not care about the contents of the context.
//
// SEE https://www.w3.org/TR/json-ld/#the-context
type Context struct {
	rawValues []any
}

// Contains returns true if the given context is contained in the context.
//
// NOTE: This only checks for membership and doesn't look for expanded context,
// etc.
func (c Context) Contains(context any) bool {
	return slices.Contains(c.rawValues, context)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (c *Context) UnmarshalJSON(data []byte) error {
	var rawValuesArray []any
	if err := json.Unmarshal(data, &rawValuesArray); err == nil {
		c.rawValues = rawValuesArray

		return nil
	}

	var rawValues any
	if err := json.Unmarshal(data, &rawValues); err != nil {
		return fmt.Errorf("unmarshal context: %w", err)
	}

	c.rawValues = []any{rawValues}

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (c Context) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(c.rawValues)
	if err != nil {
		return nil, fmt.Errorf("marshal context: %w", err)
	}

	return b, nil
}

// NewContext creates a new context from the given raw values.
func NewContext(rawValues ...any) Context {
	return Context{rawValues: rawValues}
}

const followActivityType = "Follow"
const undoActivityType = "Undo"
const createActivityType = "Create"

// A PublicKey is a public key definition as defined by the Security Vocabulary
// (https://w3c.github.io/vc-data-integrity/vocab/security/vocabulary.html#publicKey).
//
// As in other types, we do not allow for some expanded fields, such as for
// "owner".
//
// SEE https://w3c-ccg.github.io/security-vocab/contexts/security-v1.jsonld
type PublicKey struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	PublicKeyPem string `json:"publicKeyPem"`
}

// An Image is an ActivityStreams Image used to identify a user.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-image
type Image struct {
	Context Context `json:"@context"`
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	URL     string  `json:"url"`
}

// An OrderedCollection is an ActivityStreams OrderedCollection.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-orderedcollection
type OrderedCollection[T any] struct {
	Context      Context `json:"@context"`
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	TotalItems   int     `json:"totalItems"`
	First        string  `json:"first,omitempty"`
	Last         string  `json:"last,omitempty"`
	OrderedItems []T     `json:"orderedItems,omitempty"`
}

// NewCollection creates a new OrderedCollection containing the given items.
func NewCollection[T any](id string, items []T) OrderedCollection[T] {
	return OrderedCollection[T]{
		Context: NewContext(
			ActivityStreamsContext,
			MastodonContext,
		),
		Type:         "OrderedCollection",
		ID:           id,
		TotalItems:   len(items),
		OrderedItems: items,
	}
}

// An Activity is an ActivityStreams Activity.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-activity
type Activity[T any] struct {
	Context   Context  `json:"@context"`
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	Actor     string   `json:"actor,omitempty"`
	Object    T        `json:"object,omitempty"`
	Published string   `json:"published,omitempty"`
	To        []string `json:"to,omitempty"`
	Cc        []string `json:"cc,omitempty"`
}

func newAcceptActivity(actorID string, activityID string) Activity[string] {
	return Activity[string]{
		Context: NewContext(ActivityStreamsContext),
		Type:    "Accept",
		Actor:   actorID,
		Object:  activityID,
	}
}

// NewCreateActivity creates a new Create activity.
func NewCreateActivity[T any](actor ActorLike, object T, published string, to, cc []string) Activity[T] {
	return Activity[T]{
		Context:   NewContext(ActivityStreamsContext),
		Type:      createActivityType,
		ID:        ActorID(actor) + "/outbox/" + uuid.NewString(),
		Actor:     ActorID(actor),
		Object:    object,
		Published: published,
		To:        to,
		Cc:        cc,
	}
}

// A Note is an ActivityStreams Note.
//
// SEE https://www.w3.org/TR/activitystreams-vocabulary/#dfn-note
type Note struct {
	Context      Context  `json:"@context"`
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	AttributedTo string   `json:"attributedTo"`
	Content      string   `json:"content"`
	Published    string   `json:"published"`
	Sensitive    bool     `json:"sensitive"`
	To           []string `json:"to"`
	Cc           []string `json:"cc"`
}

// NewNote creates a new Note.
func NewNote(actor ActorLike, content string, to, cc []string) Note {
	return Note{
		Context:      NewContext(ActivityStreamsContext, MastodonContext),
		Type:         "Note",
		ID:           ActorID(actor) + "/notes/" + uuid.NewString(),
		AttributedTo: ActorID(actor),
		Content:      content,
		Published:    time.Now().UTC().Format(http.TimeFormat),
		To:           to,
		Cc:           cc,
	}
}

// An Actor is an ActivityPub actor.
//
// We also include Mastodon-specific fields here, such as the public key.
// Likewise, we ignore some parts of ActivityPub/ActivityStreams, such as
// natural language support
// (https://www.w3.org/TR/activitystreams-core/#naturalLanguageValues).
//
// SEE https://www.w3.org/TR/activitypub/#actor-objects
type Actor struct {
	Context                   Context            `json:"@context"`
	Type                      string             `json:"type"`
	ID                        string             `json:"id"`
	Inbox                     string             `json:"inbox,omitempty"`
	Outbox                    string             `json:"outbox,omitempty"`
	Following                 string             `json:"following,omitempty"`
	Followers                 string             `json:"followers,omitempty"`
	PreferredUsername         string             `json:"preferredUsername,omitempty"`
	Name                      string             `json:"name,omitempty"`
	Summary                   string             `json:"summary,omitempty"`
	URL                       string             `json:"url,omitempty"`
	Discoverable              bool               `json:"discoverable"`
	ManuallyApprovesFollowers bool               `json:"manuallyApprovesFollowers"`
	Icon                      Image              `json:"icon,omitempty"`
	Attachment                []SchemaAttachment `json:"attachment,omitempty"`
	PublicKey                 PublicKey          `json:"publicKey,omitempty"`
}

// An ActorLike is an interface for types that can be actors (they have
// usernames).
type ActorLike interface {
	GetName() string
	GetImageURL() string
	GetSummary() string
	GetUsername() string
	GetAttachment() orderedmap.OrderedMap
}

// ActorID gets the ID of the actor.
func ActorID(actor ActorLike) string {
	return fmt.Sprintf("https://%s/~%s", Domain, actor.GetUsername())
}

// ActorOutbox gets the outbox of the actor.
func ActorOutbox(actor ActorLike) string {
	return fmt.Sprintf("https://%s/~%s/outbox", Domain, actor.GetUsername())
}

// ActorFollowers gets the followers collection of the actor.
func ActorFollowers(actor ActorLike) string {
	return fmt.Sprintf("https://%s/~%s/followers", Domain, actor.GetUsername())
}

// ActorFollowing gets the following collection of the actor.
func ActorFollowing(actor ActorLike) string {
	return fmt.Sprintf("https://%s/~%s/following", Domain, actor.GetUsername())
}

// ActorInbox gets the inbox of the actor.
func ActorInbox(actor ActorLike) string {
	return fmt.Sprintf("https://%s/~%s/inbox", Domain, actor.GetUsername())
}

// ActorPublicKeyID gets the ID of the public key of the actor.
func ActorPublicKeyID(actor ActorLike) string {
	return ActorID(actor) + "#main-key"
}

// ActorFromUser gets an actor from a system user.
func ActorFromUser(user ActorLike, pubKey identity.SigningKey) (Actor, error) {
	username := user.GetUsername()

	var icon Image
	if user.GetImageURL() != "" {
		icon = Image{
			Context: NewContext(ActivityStreamsContext),
			Type:    "Image",
			Name:    "Photograph of @" + username,
			URL:     user.GetImageURL(),
		}
	}

	var attachment []SchemaAttachment

	if userAttachment := user.GetAttachment(); userAttachment != nil {
		attachment := make([]SchemaAttachment, len(userAttachment))

		for i, item := range userAttachment {
			attachment[i] = SchemaAttachment{
				Type:  "PropertyValue",
				Name:  item.Name,
				Value: item.Value,
			}
		}
	}

	return Actor{
		Context:                   NewContext(ActivityStreamsContext, SecurityContext),
		Type:                      "Person",
		ID:                        ActorID(user),
		Inbox:                     ActorInbox(user),
		Outbox:                    ActorOutbox(user),
		Followers:                 ActorFollowers(user),
		Following:                 ActorFollowing(user),
		PreferredUsername:         username,
		Name:                      user.GetName(),
		Summary:                   user.GetSummary(),
		Icon:                      icon,
		Discoverable:              true,
		ManuallyApprovesFollowers: false,
		Attachment:                attachment,
		PublicKey: PublicKey{
			ID:           ActorPublicKeyID(user),
			Owner:        ActorID(user),
			PublicKeyPem: pubKey.PEM,
		},
	}, nil
}

// A SchemaAttachment is a http://schema.org#PropertyValue.
type SchemaAttachment struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Value string `json:"value"`
}
