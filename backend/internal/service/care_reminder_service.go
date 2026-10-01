package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/model"
	"github.com/gbplantwiki/gbplantwiki/internal/repository"
	"github.com/gbplantwiki/gbplantwiki/internal/util"
)

// CareReminderService implements care reminder state machine logic.
// The state machine (pending -> done / overdue) is intentionally mirrored in
// frontend button visibility, log templates, error codes and formatters.
type CareReminderService struct {
	db         *gorm.DB
	repo       *repository.CareReminderRepository
	gardenRepo *repository.UserGardenRepository
	logger     *slog.Logger
}

// NewCareReminderService creates a CareReminderService.
func NewCareReminderService(db *gorm.DB, repo *repository.CareReminderRepository, gardenRepo *repository.UserGardenRepository, logger *slog.Logger) *CareReminderService {
	return &CareReminderService{db: db, repo: repo, gardenRepo: gardenRepo, logger: logger}
}

// CreateResult reports whether a plan already existed (two devices submitting
// the same plan get the existing plan back instead of a hard error).
type CreateResult struct {
	Reminder       *model.CareReminder
	AlreadyExisted bool
}

// Create adds a reminder for the current user, bound to a concrete pot.
// When an identical plan (same owner + pot + task + due date) already exists,
// the existing plan is returned with AlreadyExisted=true.
func (s *CareReminderService) Create(userID uint, m *model.CareReminder) (*CreateResult, error) {
	m.UserID = userID
	if m.Status == "" {
		m.Status = model.ReminderPending
	}
	if m.RemindDate.IsZero() {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareReminder[task_title=%s] create failed: remind_date required", m.TaskTitle))
	}

	var result *CreateResult
	for attempt := 0; attempt < uniqueRaceRetries; attempt++ {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if m.GardenID == 0 && m.PlantSpeciesID > 0 {
				pot, err := s.gardenRepoWithTx(tx).Find(userID, m.PlantSpeciesID)
				if err == nil {
					m.GardenID = pot.ID
				} else if !errors.Is(err, repository.ErrNotFound) {
					return fmt.Errorf("care reminder pot resolve: %w", err)
				}
			}
			if m.GardenID > 0 {
				pot, err := s.gardenRepoWithTx(tx).FindByID(m.GardenID)
				if err != nil {
					if errors.Is(err, repository.ErrNotFound) {
						return util.NewAppError(422, constants.CodeValidationError,
							fmt.Sprintf("CareReminder[task_title=%s] create failed: garden_id=%d not found", m.TaskTitle, m.GardenID))
					}
					return fmt.Errorf("care reminder pot verify: %w", err)
				}
				if pot.UserID != userID {
					return util.NewAppError(403, constants.CodeForbidden,
						fmt.Sprintf("CareReminder[task_title=%s] create failed: not pot owner", m.TaskTitle))
				}
				// Keep denormalized fields in sync with the bound pot.
				m.PlantSpeciesID = pot.PlantSpeciesID
			}
			// Dedup identical plans (same owner + pot + task + due date).
			// This also covers generic reminders sharing garden_id = 0.
			existing, err := s.repo.FindPlan(tx, userID, m.GardenID, m.TaskTitle, m.RemindDate)
			if err == nil {
				result = &CreateResult{Reminder: existing, AlreadyExisted: true}
				return nil
			}
			if !errors.Is(err, repository.ErrNotFound) {
				return fmt.Errorf("care reminder plan lookup: %w", err)
			}
			if err := s.repo.CreateTx(tx, m); err != nil {
				if errors.Is(err, repository.ErrDuplicate) {
					// Another device won the race against uk_reminder_plan.
					// Our transaction snapshot cannot see its uncommitted row,
					// so roll back and let the outer loop re-read with a fresh
					// snapshot.
					return errUniqueRace
				}
				s.logger.Error(fmt.Sprintf(constants.LogReminderCreateFailed, m.TaskTitle), "error", err)
				return fmt.Errorf("care reminder create: %w", err)
			}
			result = &CreateResult{Reminder: m, AlreadyExisted: false}
			return nil
		})
		if err == nil {
			if !result.AlreadyExisted {
				s.logger.Info(fmt.Sprintf(constants.LogReminderCreateSuccess, m.TaskTitle), "id", result.Reminder.ID)
			} else {
				s.logger.Info(fmt.Sprintf(constants.LogReminderPlanExists, m.TaskTitle), "id", result.Reminder.ID)
			}
			return result, nil
		}
		if !errors.Is(err, errUniqueRace) {
			return nil, err
		}
		// Fresh-snapshot read with a short wait: the winning device may still
		// be committing. If the blocker disappeared (winner rolled back), loop
		// and insert again.
		existing, ferr := s.waitForPlan(userID, m.GardenID, m.TaskTitle, m.RemindDate)
		if ferr == nil {
			s.logger.Info(fmt.Sprintf(constants.LogReminderPlanExists, m.TaskTitle), "id", existing.ID)
			return &CreateResult{Reminder: existing, AlreadyExisted: true}, nil
		}
		if !errors.Is(ferr, repository.ErrNotFound) {
			return nil, fmt.Errorf("care reminder plan race refind: %w", ferr)
		}
	}
	return nil, fmt.Errorf("care reminder create: plan contention not settled after %d attempts", uniqueRaceRetries)
}

// waitForPlan reads an identical plan with a fresh snapshot, polling briefly
// for a concurrently committing transaction to become visible.
func (s *CareReminderService) waitForPlan(userID, gardenID uint, taskTitle string, remindDate time.Time) (*model.CareReminder, error) {
	var lastErr error
	for i := 0; i < planWaitRetries; i++ {
		existing, err := s.repo.FindPlan(s.db, userID, gardenID, taskTitle, remindDate)
		if err == nil {
			return existing, nil
		}
		lastErr = err
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		time.Sleep(planWaitInterval)
	}
	return nil, lastErr
}

// gardenRepoWithTx returns a pot repository bound to a transaction so all
// reads inside a plan creation see the same snapshot.
func (s *CareReminderService) gardenRepoWithTx(tx *gorm.DB) *repository.UserGardenRepository {
	return repository.NewUserGardenRepository(tx)
}

// attachLegacyReminders attributes unbound legacy reminders of a user to the
// earliest-taken-over pot of their species ("旧提醒没有花盆编号时归到该品种
// 最早接管的那盆"). Runs in one transaction so it cannot leave half-updated
// rows.
func (s *CareReminderService) attachLegacyReminders(userID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		unbound, err := s.repo.ListUnboundByUser(tx, userID)
		if err != nil {
			return fmt.Errorf("legacy reminder list: %w", err)
		}
		for i := range unbound {
			r := &unbound[i]
			pot, err := s.gardenRepoWithTx(tx).Find(r.UserID, r.PlantSpeciesID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					continue // species no longer in garden: leave generic
				}
				return fmt.Errorf("legacy reminder pot resolve: %w", err)
			}
			if err := s.repo.BindToGardenTx(tx, r.ID, pot.ID); err != nil {
				return fmt.Errorf("legacy reminder bind: %w", err)
			}
		}
		return nil
	})
}

// ListByUser lists reminders with status filter.
func (s *CareReminderService) ListByUser(userID uint, status string) ([]model.CareReminder, error) {
	if err := s.attachLegacyReminders(userID); err != nil {
		s.logger.Warn("legacy reminder attachment failed", "error", err)
	}
	if _, err := s.repo.MarkOverdue(userID); err != nil {
		s.logger.Warn("care reminder overdue mark failed", "error", err)
	}
	items, err := s.repo.ListByUser(userID, status)
	if err != nil {
		return nil, fmt.Errorf("care reminder list: %w", err)
	}
	return items, nil
}

// ListByMonth lists reminders within a calendar month.
func (s *CareReminderService) ListByMonth(userID uint, year, month int) ([]model.CareReminder, error) {
	if err := s.attachLegacyReminders(userID); err != nil {
		s.logger.Warn("legacy reminder attachment failed", "error", err)
	}
	items, err := s.repo.ListByMonth(userID, year, month)
	if err != nil {
		return nil, fmt.Errorf("care reminder month list: %w", err)
	}
	return items, nil
}

// UpdateStatus transitions a reminder to a new status.
func (s *CareReminderService) UpdateStatus(userID, id uint, status string) (*model.CareReminder, error) {
	m, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareReminder[id=%d] not found", id))
		}
		return nil, fmt.Errorf("care reminder status find: %w", err)
	}
	if m.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("CareReminder[id=%d] status change failed: user_id=%d not owner", id, userID))
	}
	switch status {
	case model.ReminderDone:
		m.Status = model.ReminderDone
	case model.ReminderPending:
		if m.RemindDate.Before(time.Now()) {
			m.Status = model.ReminderOverdue
		} else {
			m.Status = model.ReminderPending
		}
	default:
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareReminder[id=%d] status=%s invalid transition", id, status))
	}
	if err := s.repo.Update(m); err != nil {
		return nil, fmt.Errorf("care reminder status update: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogReminderStatusChanged, id, m.Status), "id", id)
	return m, nil
}

// Delete removes a reminder owned by the user.
func (s *CareReminderService) Delete(userID, id uint) error {
	m, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("care reminder delete find: %w", err)
	}
	if m.UserID != userID {
		return util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("CareReminder[id=%d] delete failed: not owner", id))
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("care reminder delete: %w", err)
	}
	return nil
}
