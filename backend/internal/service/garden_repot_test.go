package service

import (
	"testing"
	"time"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return d
}

func TestRescheduleDate(t *testing.T) {
	cases := []struct {
		name      string
		oldDate   string
		oldAnchor string
		newAnchor string
		frequency string
		want      string
	}{
		{
			name:      "shift future occurrence by day delta",
			oldDate:   "2026-04-10",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-02-01",
			frequency: model.FrequencyMonthly,
			want:      "2026-05-11",
		},
		{
			name:      "past occurrence rolls forward to first occurrence at/after new anchor",
			oldDate:   "2026-01-15",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-06-01",
			frequency: model.FrequencyMonthly,
			// shifted = 2026-06-15, already >= new anchor
			want: "2026-06-15",
		},
		{
			name:      "weekly old occurrence before new anchor rolls forward by weeks",
			oldDate:   "2026-01-03",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-02-01",
			frequency: model.FrequencyWeekly,
			// shifted = 2026-02-03 (>= 2026-02-01), no extra roll
			want: "2026-02-03",
		},
		{
			name:      "daily rolls forward until not before new anchor",
			oldDate:   "2026-01-02",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-01-10",
			frequency: model.FrequencyDaily,
			want:      "2026-01-11",
		},
		{
			name:      "one-off reminder is only shifted, never rolled",
			oldDate:   "2026-01-02",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-03-01",
			frequency: model.FrequencyNone,
			want:      "2026-03-02",
		},
		{
			name:      "same anchor keeps date",
			oldDate:   "2026-05-20",
			oldAnchor: "2026-01-01",
			newAnchor: "2026-01-01",
			frequency: model.FrequencyMonthly,
			want:      "2026-05-20",
		},
		{
			name:      "backward repot shifts earlier by day delta",
			oldDate:   "2026-06-15",
			oldAnchor: "2026-03-01",
			newAnchor: "2026-02-01",
			frequency: model.FrequencyMonthly,
			want:      "2026-05-18",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rescheduleDate(
				mustDate(t, tc.oldDate),
				mustDate(t, tc.oldAnchor),
				mustDate(t, tc.newAnchor),
				tc.frequency,
			)
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("got %s, want %s", got.Format("2006-01-02"), tc.want)
			}
		})
	}
}

func TestStatusForDate(t *testing.T) {
	today := dateOnly(time.Now())
	if got := statusForDate(today.AddDate(0, 0, -1)); got != model.ReminderOverdue {
		t.Fatalf("past date: got %s, want overdue", got)
	}
	if got := statusForDate(today); got != model.ReminderPending {
		t.Fatalf("today: got %s, want pending", got)
	}
	if got := statusForDate(today.AddDate(0, 0, 3)); got != model.ReminderPending {
		t.Fatalf("future date: got %s, want pending", got)
	}
}

func TestDateOnly(t *testing.T) {
	d := dateOnly(time.Date(2026, 3, 7, 15, 30, 0, 0, time.Local))
	if d.Hour() != 0 || d.Minute() != 0 || d.Day() != 7 {
		t.Fatalf("dateOnly did not truncate: %v", d)
	}
}
