package util

import (
	"strings"
	"time"
)

// DateLayout is the wire format for date-only payloads ("YYYY-MM-DD").
const DateLayout = "2006-01-02"

// Date is a time.Time wrapper that (un)marshals date-only JSON. The frontend
// date pickers send "2026-01-02", which the standard time.Time cannot parse.
type Date time.Time

// UnmarshalJSON accepts "YYYY-MM-DD" and RFC3339 date-time strings.
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*d = Date(time.Time{})
		return nil
	}
	if t, err := time.ParseInLocation(DateLayout, s, time.Local); err == nil {
		*d = Date(t)
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// MarshalJSON renders the local calendar day only.
func (d Date) MarshalJSON() ([]byte, error) {
	t := time.Time(d)
	if t.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + t.Format(DateLayout) + `"`), nil
}

// Time returns the underlying time.Time.
func (d Date) Time() time.Time { return time.Time(d) }

// IsZero reports whether the date carries no value.
func (d Date) IsZero() bool { return time.Time(d).IsZero() }
