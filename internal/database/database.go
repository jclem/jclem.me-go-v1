package database

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
)

// A ULID is a wrapper around ulid.ULID that implements the sql.Scanner and
// driver.Valuer interfaces.
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

// String implements the fmt.Stringer interface.
func (u ULID) String() string {
	return strings.ToLower(ulid.ULID(u).String())
}
