package service

import (
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newMockGorm(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db, mock
}

// When two devices submit the same plan concurrently and the INSERT loses the
// race against the unique index, the later request must return the existing
// plan with AlreadyExisted=true instead of failing.
func TestCareReminderCreateConcurrentReturnsExisting(t *testing.T) {
	db, mock := newMockGorm(t)
	svc := NewCareReminderService(db, repository.NewCareReminderRepository(db), repository.NewUserGardenRepository(db), testLogger(t))

	due := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	// pot verification: SELECT * FROM user_gardens WHERE id = ?
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "pot_no", "status"}).
			AddRow(7, 42, 4, "P0001", "active"))
	// identical plan lookup returns nothing
	mock.ExpectQuery("SELECT \\* FROM `care_reminders`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	// concurrent insert loses the race
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `care_reminders`")).
		WillReturnError(errors.New("Duplicate entry '42-7-give water-2026-10-05' for key 'uk_reminder_plan'"))
	mock.ExpectRollback()
	// fresh-snapshot poll outside the transaction finds the winner
	mock.ExpectQuery("SELECT \\* FROM `care_reminders`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "garden_id", "plant_species_id", "task_title", "status"}).
			AddRow(99, 42, 7, 4, "give water", "pending"))

	m := &model.CareReminder{GardenID: 7, TaskTitle: "give water", RemindDate: due, Frequency: "weekly"}
	res, err := svc.Create(42, m)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !res.AlreadyExisted || res.Reminder.ID != 99 {
		t.Errorf("expected existing plan id=99 with AlreadyExisted, got %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

// Legacy reminders without a pot number are attached to the earliest-taken-over
// pot of their species.
func TestAttachLegacyRemindersUsesEarliestPot(t *testing.T) {
	db, mock := newMockGorm(t)
	reminderRepo := repository.NewCareReminderRepository(db)
	gardenRepo := repository.NewUserGardenRepository(db)
	svc := NewCareReminderService(db, reminderRepo, gardenRepo, testLogger(t))

	mock.ExpectBegin()
	// list unbound legacy reminders (GORM inlines the zero thresholds)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_reminders` WHERE user_id = ? AND garden_id = 0 AND plant_species_id > 0")).
		WithArgs(uint(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "garden_id", "plant_species_id", "task_title", "status"}).
			AddRow(5, 42, 0, 4, "old task", "pending"))
	// earliest pot of species 4 (ORDER BY owned_since ASC, id ASC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE user_id = ? AND plant_species_id = ? ORDER BY owned_since ASC, id ASC,`user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(42), uint(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "pot_no", "owned_since", "status"}).
			AddRow(11, 42, 4, "P0001", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), "active"))
	// bind reminder 5 -> pot 11
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `care_reminders` SET `garden_id`=? WHERE id = ?")).
		WithArgs(uint(11), uint(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := svc.attachLegacyReminders(42); err != nil {
		t.Fatalf("attachLegacyReminders: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}
