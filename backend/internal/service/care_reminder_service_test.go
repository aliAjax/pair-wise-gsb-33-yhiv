package service

import (
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func newRepotServiceMock(t *testing.T) (sqlmock.Sqlmock, *CareReminderService) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db := openMockGorm(t, sqlDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reminderRepo := repository.NewCareReminderRepository(db)
	gardenRepo := repository.NewUserGardenRepository(db)
	return mock, NewCareReminderService(db, reminderRepo, gardenRepo, logger)
}

func TestCreateReminderDuplicateReturnsExistingPlan409(t *testing.T) {
	mock, svc := newRepotServiceMock(t)
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local)

	mock.ExpectBegin()
	// ownership lookup of the pot succeeds
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(5), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "nickname", "location"}).
			AddRow(5, 2, 4, "月季·老桩", "南阳台 A1"))
	// insert hits the plan unique index (two devices racing)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `care_reminders`")).
		WillReturnError(errors.New("Error 1062 (23000): Duplicate entry for key 'uk_reminder_plan'"))
	mock.ExpectRollback()
	// service loads the already-stored plan to return it
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `care_reminders` WHERE user_id = ? AND user_garden_id = ? AND task_title = ? AND remind_date = ? ORDER BY `care_reminders`.`id` LIMIT ?")).
		WithArgs(uint(2), uint(5), "浇水", date, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "user_garden_id", "plant_species_id", "task_title", "remind_date", "frequency", "status"}).
			AddRow(99, 2, 5, 4, "浇水", date, "weekly", "pending"))
	// then builds the enriched view with pot display data
	mock.ExpectQuery("(?s)SELECT .*FROM care_reminders LEFT JOIN user_gardens.*").
		WithArgs(uint(99)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "user_garden_id", "plant_species_id", "task_title",
			"remind_date", "frequency", "status", "created_at",
			"pot_number", "pot_nickname", "pot_location",
		}).AddRow(99, 2, 5, 4, "浇水", date, "weekly", "pending", time.Now(),
			5, "月季·老桩", "南阳台 A1"))

	m := &model.CareReminder{UserGardenID: 5, TaskTitle: "浇水", RemindDate: date, Frequency: "weekly"}
	view, err := svc.Create(2, m)
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if view != nil {
		t.Fatalf("late request must not create a plan, got view %+v", view)
	}
	if !isAppStatus(err, 409) {
		t.Fatalf("expected 409 AppError, got %v", err)
	}
	if existing := appErrorData(t, err); existing.ID != 99 || existing.PotNumber != 5 {
		t.Fatalf("409 must carry the existing plan, got %+v", existing)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateReminderRejectsMissingPot(t *testing.T) {
	_, svc := newRepotServiceMock(t)
	_, err := svc.Create(2, &model.CareReminder{TaskTitle: "浇水", RemindDate: time.Now()})
	if !isAppStatus(err, 422) {
		t.Fatalf("expected 422 without pot id, got %v", err)
	}
}

func TestCreateReminderRejectsBadFrequency(t *testing.T) {
	_, svc := newRepotServiceMock(t)
	_, err := svc.Create(2, &model.CareReminder{UserGardenID: 5, TaskTitle: "浇水", RemindDate: time.Now(), Frequency: "hourly"})
	if !isAppStatus(err, 422) {
		t.Fatalf("expected 422 for bad frequency, got %v", err)
	}
}
