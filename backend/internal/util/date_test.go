package util

import "testing"

func TestDateUnmarshalDateOnly(t *testing.T) {
	var d Date
	if err := d.UnmarshalJSON([]byte(`"2026-09-01"`)); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := d.Time()
	if got.Year() != 2026 || got.Month() != 9 || got.Day() != 1 {
		t.Fatalf("unexpected date: %v", got)
	}
	if out, _ := d.MarshalJSON(); string(out) != `"2026-09-01"` {
		t.Fatalf("marshal = %s", string(out))
	}
}

func TestDateUnmarshalEmpty(t *testing.T) {
	var d Date
	if err := d.UnmarshalJSON([]byte(`""`)); err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if !d.IsZero() {
		t.Fatalf("expected zero date, got %v", d.Time())
	}
	var d2 Date
	if err := d2.UnmarshalJSON([]byte(`"2026-09-01T08:30:00+08:00"`)); err != nil {
		t.Fatalf("unmarshal rfc3339: %v", err)
	}
}
