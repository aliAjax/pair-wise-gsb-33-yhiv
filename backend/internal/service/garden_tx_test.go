package service

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/gbplantwiki/gbplantwiki/internal/repository"
)

func errBoom() error { return errors.New("boom") }

// Removing a pot deletes its reminders first; when the pot delete fails the
// whole transaction rolls back so neither side keeps half the data.
func TestGardenRemoveRollsBackOnPotDeleteFailure(t *testing.T) {
	db, mock := newMockGorm(t)
	svc := NewUserGardenService(db, repository.NewUserGardenRepository(db), repository.NewCareReminderRepository(db), testLogger(t))

	mock.ExpectBegin()
	// ownership load inside the transaction
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(3), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "pot_no", "status"}).
			AddRow(3, 42, 4, "P0003", "active"))
	// reminders deleted
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `care_reminders` WHERE garden_id = ?")).
		WithArgs(uint(3)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	// pot delete fails
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `user_gardens` WHERE `user_gardens`.`id` = ?")).
		WithArgs(uint(3)).
		WillReturnError(errBoom())
	mock.ExpectRollback()

	if err := svc.Remove(42, 3); err == nil {
		t.Fatal("expected error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

// Repotting rolls back when rescheduling reminders fails: the new pot must not
// survive and the old pot must stay active.
func TestRepotRollsBackOnRescheduleFailure(t *testing.T) {
	db, mock := newMockGorm(t)
	svc := NewUserGardenService(db, repository.NewUserGardenRepository(db), repository.NewCareReminderRepository(db), testLogger(t))

	// input validation load before the transaction
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "pot_no", "owned_since", "location", "status"}).
			AddRow(1, 42, 4, "P0001", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), "南阳台", "active"))
	mock.ExpectBegin()
	// reload inside transaction
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_gardens` WHERE `user_gardens`.`id` = ? ORDER BY `user_gardens`.`id` LIMIT ?")).
		WithArgs(uint(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "plant_species_id", "pot_no", "owned_since", "location", "status"}).
			AddRow(1, 42, 4, "P0001", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), "南阳台", "active"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `user_gardens` WHERE user_id = ?")).
		WithArgs(uint(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `user_gardens`")).
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectExec("UPDATE `care_reminders` SET .*garden_id").
		WillReturnError(errBoom())
	mock.ExpectRollback()

	_, err := svc.Repot(42, 1, "", "东阳台", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected repot error, got nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}
