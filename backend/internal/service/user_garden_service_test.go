package service

import (
	"testing"
	"time"
)

func TestNextPotNo(t *testing.T) {
	cases := []struct {
		existing int64
		want     string
	}{
		{0, "P0001"},
		{1, "P0002"},
		{12, "P0013"},
		{9999, "P10000"},
	}
	for _, c := range cases {
		if got := nextPotNo(c.existing); got != c.want {
			t.Errorf("nextPotNo(%d) = %s, want %s", c.existing, got, c.want)
		}
	}
}

func TestRepotDeltaDays(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	old := time.Date(2026, 3, 1, 9, 30, 0, 0, loc)
	new := time.Date(2026, 4, 10, 7, 0, 0, 0, loc)
	// Time-of-day differences must not affect the whole-day shift.
	if got := repotDeltaDays(old, new); got != 40 {
		t.Errorf("repotDeltaDays = %d, want 40", got)
	}
	same := time.Date(2026, 3, 1, 20, 0, 0, 0, loc)
	if got := repotDeltaDays(old, same); got != 0 {
		t.Errorf("repotDeltaDays same day = %d, want 0", got)
	}
}
