package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

func TestReminderCreateDuplicateMapsToSentinel(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareReminderRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `care_reminders`")).
		WillReturnError(errors.New("Error 1062 (23000): Duplicate entry '2-5' for key 'care_reminders.uk_reminder_plan'"))
	mock.ExpectRollback()
	err := repo.Create(&model.CareReminder{UserID: 2, UserGardenID: 5, TaskTitle: "浇水", RemindDate: time.Now()})
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
}

func TestReminderListViewJoinsPot(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareReminderRepository(db)
	remindDate := time.Date(2026, 10, 3, 0, 0, 0, 0, time.Local)
	mock.MatchExpectationsInOrder(false)
	// MarkOverdue runs before the list query.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `care_reminders` SET").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("(?s)SELECT .*FROM care_reminders LEFT JOIN user_gardens.*").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_garden_id", "plant_species_id", "task_title",
			"remind_date", "frequency", "status", "created_at",
			"pot_number", "pot_nickname", "pot_location",
		}).AddRow(10, 2, 5, 4, "给月季施肥", remindDate, "monthly", "pending", time.Now(),
			5, "月季·老桩", "南阳台 A1"))

	if _, err := repo.MarkOverdue(2); err != nil {
		t.Fatalf("MarkOverdue: %v", err)
	}
	views, err := repo.ListByUser(2, "")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	v := views[0]
	if v.PotNumber != 5 || v.PotNickname != "月季·老桩" || v.PotLocation != "南阳台 A1" || v.UserGardenID != 5 {
		t.Fatalf("pot enrichment missing: %+v", v)
	}
	if v.TaskTitle != "给月季施肥" || v.Frequency != "monthly" {
		t.Fatalf("unexpected view payload: %+v", v)
	}
}

func TestReminderListViewKeepsLegacyUnboundRows(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareReminderRepository(db)
	remindDate := time.Date(2026, 10, 3, 0, 0, 0, 0, time.Local)
	mock.ExpectQuery("(?s)SELECT .*LEFT JOIN user_gardens.*").
		WithArgs(uint(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_garden_id", "plant_species_id", "task_title",
			"remind_date", "frequency", "status", "created_at",
			"pot_number", "pot_nickname", "pot_location",
		}).AddRow(11, 2, 0, 4, "旧提醒", remindDate, "", "pending", time.Now(),
			0, "", ""))
	views, err := repo.ListByUser(2, "")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(views) != 1 || views[0].UserGardenID != 0 || views[0].PotNumber != 0 {
		t.Fatalf("legacy reminder should stay visible unbound: %+v", views)
	}
}

func TestAssignLegacyRemindersSQL(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewCareReminderRepository(db)
	mock.ExpectExec("(?s)UPDATE care_reminders r\\s+SET r\\.user_garden_id = .*ORDER BY g\\.owned_since ASC, g\\.id ASC.*").
		WillReturnResult(sqlmock.NewResult(0, 3))
	if err := repo.AssignLegacyReminders(); err != nil {
		t.Fatalf("AssignLegacyReminders: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestFindEarliestPot(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewUserGardenRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE user_id = ? AND plant_species_id = ? ORDER BY owned_since ASC, id ASC,`user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(2), uint(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "nickname", "location"}).
			AddRow(7, 2, 4, "月季·老桩", "南阳台 A1"))
	pot, err := repo.FindEarliestPot(2, 4)
	if err != nil {
		t.Fatalf("FindEarliestPot: %v", err)
	}
	if pot.ID != 7 {
		t.Fatalf("unexpected pot: %+v", pot)
	}
}
