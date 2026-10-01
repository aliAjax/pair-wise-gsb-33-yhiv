package service

import (
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func newGardenServiceMock(t *testing.T) (sqlmock.Sqlmock, *UserGardenService) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db := openMockGorm(t, sqlDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gardenRepo := repository.NewUserGardenRepository(db)
	reminderRepo := repository.NewCareReminderRepository(db)
	return mock, NewUserGardenService(db, gardenRepo, reminderRepo, logger)
}

func TestRepotReschedulesUnfinishedAndKeepsDone(t *testing.T) {
	mock, svc := newGardenServiceMock(t)
	oldAnchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	newAnchor := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)

	pendingDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local) // -> +31d = 2026-04-10
	doneDate := time.Date(2026, 1, 20, 0, 0, 0, 0, time.Local)    // done: untouched
	overdueDate := time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local)  // -> 2026-02-05 pending

	mock.ExpectBegin()
	// load pot
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "nickname", "owned_since", "location"}).
			AddRow(7, 2, 4, "月季·老桩", oldAnchor, "南阳台 A1"))
	// list reminders of the pot
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_reminders` WHERE user_garden_id = ? ORDER BY remind_date ASC, id ASC")).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_garden_id", "plant_species_id", "task_title", "remind_date", "frequency", "status"}).
			AddRow(101, 2, 7, 4, "施肥", pendingDate, "monthly", "pending").
			AddRow(102, 2, 7, 4, "修剪", doneDate, "monthly", "done").
			AddRow(103, 2, 7, 4, "浇水", overdueDate, "monthly", "overdue"))
	// pending reminder updated to 2026-04-10
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `care_reminders` SET")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// overdue reminder updated to 2026-02-05
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `care_reminders` SET")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// pot itself saved with new anchor
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `user_gardens` SET")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// reload pot and enriched views for the two updated reminders
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "nickname", "owned_since", "location"}).
			AddRow(7, 2, 4, "月季·老桩", newAnchor, "南阳台 A1"))
	mock.ExpectQuery("(?s)SELECT .*FROM care_reminders LEFT JOIN user_gardens.*").
		WithArgs(uint(101)).
		WillReturnRows(viewRow(101, "施肥", pendingDate.AddDate(0, 0, 31), "pending"))
	mock.ExpectQuery("(?s)SELECT .*FROM care_reminders LEFT JOIN user_gardens.*").
		WithArgs(uint(102)).
		WillReturnRows(viewRow(102, "修剪", doneDate, "done"))
	mock.ExpectQuery("(?s)SELECT .*FROM care_reminders LEFT JOIN user_gardens.*").
		WithArgs(uint(103)).
		WillReturnRows(viewRow(103, "浇水", overdueDate.AddDate(0, 0, 31), "pending"))
	mock.ExpectCommit()

	view, err := svc.Repot(2, 7, newAnchor, nil, nil)
	if err != nil {
		t.Fatalf("Repot: %v", err)
	}
	if !view.OwnedSince.Equal(newAnchor) {
		t.Fatalf("new anchor not stored: %v", view.OwnedSince)
	}
	if len(view.Reminders) != 3 {
		t.Fatalf("all pot reminders (incl. untouched done one) must be returned, got %d", len(view.Reminders))
	}
	done := view.Reminders[0]
	for _, r := range view.Reminders {
		if r.ID == 102 {
			done = r
		}
	}
	if done.ID != 102 || done.RemindDate.Format("2006-01-02") != "2026-01-20" || done.Status != "done" {
		t.Fatalf("done record must keep its original date/status: %+v", done)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRepotRollsBackWhenReminderUpdateFails(t *testing.T) {
	mock, svc := newGardenServiceMock(t)
	oldAnchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	newAnchor := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "nickname", "owned_since", "location"}).
			AddRow(7, 2, 4, "月季·老桩", oldAnchor, "南阳台 A1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_reminders` WHERE user_garden_id = ? ORDER BY remind_date ASC, id ASC")).
		WithArgs(uint(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_garden_id", "plant_species_id", "task_title", "remind_date", "frequency", "status"}).
			AddRow(101, 2, 7, 4, "施肥", time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local), "monthly", "pending"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `care_reminders` SET")).
		WillReturnError(errBroken)
	mock.ExpectRollback()

	if _, err := svc.Repot(2, 7, newAnchor, nil, nil); err == nil {
		t.Fatal("expected error when reminder update fails")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func viewRow(id uint, title string, date time.Time, status string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "user_id", "user_garden_id", "plant_species_id", "task_title",
		"remind_date", "frequency", "status", "created_at",
		"pot_number", "pot_nickname", "pot_location",
	}).AddRow(id, 2, 7, 4, title, date, "monthly", status, time.Now(),
		7, "月季·老桩", "南阳台 A1")
}

var errBroken = errors.New("database connection lost")
