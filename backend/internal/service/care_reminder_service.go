package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/constants"
	"github.com/gbplantwiki/gbplantwiki/internal/dto"
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

// validFrequencies lists accepted reminder frequencies.
var validFrequencies = map[string]bool{
	model.FrequencyNone:    true,
	model.FrequencyDaily:   true,
	model.FrequencyWeekly:  true,
	model.FrequencyMonthly: true,
	model.FrequencyYearly:  true,
}

// Create adds a reminder for the current user and binds it to a concrete pot.
// When two devices submit the very same plan (same user, pot, title and date)
// concurrently, the database unique index rejects the late request; it then
// receives HTTP 409 together with the plan that was already stored.
func (s *CareReminderService) Create(userID uint, m *model.CareReminder) (*dto.ReminderView, error) {
	m.UserID = userID
	if m.UserGardenID == 0 {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			"CareReminder create failed: user_garden_id required")
	}
	if !validFrequencies[m.Frequency] {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareReminder[task_title=%s] create failed: invalid frequency=%s", m.TaskTitle, m.Frequency))
	}
	if m.RemindDate.IsZero() {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("CareReminder[task_title=%s] create failed: remind_date required", m.TaskTitle))
	}
	if m.Status == "" {
		m.Status = model.ReminderPending
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		pot, err := s.gardenRepo.FindByID(m.UserGardenID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound,
					fmt.Sprintf("UserGarden[id=%d] not found", m.UserGardenID))
			}
			return fmt.Errorf("care reminder create pot find: %w", err)
		}
		if pot.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("CareReminder create failed: pot_id=%d not owned by user_id=%d", m.UserGardenID, userID))
		}
		m.PlantSpeciesID = pot.PlantSpeciesID
		if err := s.repo.CreateTx(tx, m); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			existing, findErr := s.repo.FindPlan(userID, m.UserGardenID, m.TaskTitle, m.RemindDate)
			if findErr != nil {
				return nil, fmt.Errorf("care reminder duplicate lookup: %w", findErr)
			}
			view, viewErr := s.viewOf(existing)
			if viewErr != nil {
				return nil, viewErr
			}
			return nil, util.NewAppError(409, constants.CodeConflict,
				"该养护计划已存在，返回已有计划").WithData(view)
		}
		var appErr *util.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		s.logger.Error(fmt.Sprintf(constants.LogReminderCreateFailed, m.TaskTitle), "error", err)
		return nil, fmt.Errorf("care reminder create: %w", err)
	}

	view, err := s.viewOf(m)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogReminderCreateSuccess, m.TaskTitle), "id", m.ID, "pot_id", m.UserGardenID)
	return &view, nil
}

// BindToPot associates an existing reminder with a pot. Both writes run in one
// transaction; a failure rolls back so neither side keeps half-applied data.
func (s *CareReminderService) BindToPot(userID, gardenID, reminderID uint) (*dto.ReminderView, error) {
	var view dto.ReminderView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		pot, err := s.gardenRepo.FindByID(gardenID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("UserGarden[id=%d] not found", gardenID))
			}
			return fmt.Errorf("care reminder bind pot find: %w", err)
		}
		if pot.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("CareReminder[id=%d] bind failed: pot_id=%d not owner", reminderID, gardenID))
		}
		reminder, err := s.repo.FindByID(reminderID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareReminder[id=%d] not found", reminderID))
			}
			return fmt.Errorf("care reminder bind find: %w", err)
		}
		if reminder.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("CareReminder[id=%d] bind failed: not owner", reminderID))
		}
		reminder.UserGardenID = gardenID
		reminder.PlantSpeciesID = pot.PlantSpeciesID
		if err := s.repo.UpdateTx(tx, reminder); err != nil {
			return fmt.Errorf("care reminder bind update: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(reminderID)
	if err != nil {
		return nil, fmt.Errorf("care reminder bind reload: %w", err)
	}
	view, err = s.viewOf(updated)
	if err != nil {
		return nil, err
	}
	s.logger.Info("care reminder bound to pot", "id", reminderID, "pot_id", gardenID)
	return &view, nil
}

// viewOf renders one reminder enriched with its pot display fields.
func (s *CareReminderService) viewOf(m *model.CareReminder) (dto.ReminderView, error) {
	view, err := s.repo.GetViewByID(m.ID)
	if err != nil {
		return dto.ReminderView{}, fmt.Errorf("care reminder view: %w", err)
	}
	return *view, nil
}

// ListByUser lists reminders with status filter.
func (s *CareReminderService) ListByUser(userID uint, status string) ([]dto.ReminderView, error) {
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
func (s *CareReminderService) ListByMonth(userID uint, year, month int) ([]dto.ReminderView, error) {
	if _, err := s.repo.MarkOverdue(userID); err != nil {
		s.logger.Warn("care reminder overdue mark failed", "error", err)
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
