package models

import (
	"fmt"
	"strings"
	"time"
)

// FlexDate is a time.Time wrapper whose JSON unmarshaling accepts both
// the full RFC3339 format ("2026-04-01T00:00:00Z") and the shorter
// date-only format ("2026-04-01"). The API accepts either; the DB always
// receives a proper time.Time.
type FlexDate struct{ time.Time }

func (d *FlexDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" {
		return nil
	}
	// Try RFC3339 first (includes timezone info)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		d.Time = t
		return nil
	}
	// Fall back to date-only; interpret as midnight UTC
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("cannot parse %q as a date (use YYYY-MM-DD or RFC3339)", s)
	}
	d.Time = t
	return nil
}

func (d FlexDate) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.UTC().Format(time.RFC3339) + `"`), nil
}
