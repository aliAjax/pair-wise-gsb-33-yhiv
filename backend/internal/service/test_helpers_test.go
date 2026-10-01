package service

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/gbplantwiki/gbplantwiki/internal/dto"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

func openMockGorm(t *testing.T, sqlDB *sql.DB) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return db
}

func isAppStatus(err error, status int) bool {
	var appErr *util.AppError
	return errors.As(err, &appErr) && appErr.HTTPStatus == status
}

func appErrorData(t *testing.T, err error) dto.ReminderView {
	t.Helper()
	var appErr *util.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("not an AppError: %v", err)
	}
	view, ok := appErr.Data.(dto.ReminderView)
	if !ok {
		t.Fatalf("error data is not ReminderView: %T", appErr.Data)
	}
	return view
}

// keep sqlmock import when helper set grows in future tests
var _ = sqlmock.New
