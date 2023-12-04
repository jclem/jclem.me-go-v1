package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
)

// A ULID is a wrapper around ulid.ULID that implements the sql.Scanner and
// driver.Valuer interfaces, as well as the json.Marshaler and json.Unmarshaler
// interfaces.
type ULID ulid.ULID

// Scan implements the sql.Scanner interface.
func (u *ULID) Scan(src any) error {
	var uu ulid.ULID
	if err := uu.Scan(src); err != nil {
		return fmt.Errorf("could not scan ULID: %w", err)
	}

	*u = ULID(uu)

	return nil
}

// Value implements the driver.Valuer interface.
func (u ULID) Value() (driver.Value, error) {
	return u.String(), nil
}

// MarshalJSON implements the json.Marshaler interface.
func (u ULID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.String() + `"`), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (u *ULID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("could not unmarshal ULID: %w", err)
	}

	uu, err := ParseULID(s)
	if err != nil {
		return fmt.Errorf("could not parse ULID: %w", err)
	}

	*u = uu

	return nil
}

// String implements the fmt.Stringer interface.
func (u ULID) String() string {
	return strings.ToLower(ulid.ULID(u).String())
}

// ParseULID parses a ULID from a string.
func ParseULID(s string) (ULID, error) {
	u, err := ulid.Parse(s)
	if err != nil {
		return ULID{}, fmt.Errorf("could not parse ULID: %w", err)
	}

	return ULID(u), nil
}

// NewULID generates a new ULID.
func NewULID() ULID {
	return ULID(ulid.Make())
}
