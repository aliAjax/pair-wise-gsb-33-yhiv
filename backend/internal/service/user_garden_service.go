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

// UserGardenService implements "my garden" list logic. Each row is one
// independent pot; several pots of the same species are allowed.
type UserGardenService struct {
	db           *gorm.DB
	repo         *repository.UserGardenRepository
	reminderRepo *repository.CareReminderRepository
	logger       *slog.Logger
}

// NewUserGardenService creates a UserGardenService.
func NewUserGardenService(db *gorm.DB, repo *repository.UserGardenRepository, reminderRepo *repository.CareReminderRepository, logger *slog.Logger) *UserGardenService {
	return &UserGardenService{db: db, repo: repo, reminderRepo: reminderRepo, logger: logger}
}

// Add registers a new pot for a user. The same plant species may appear in
// multiple pots, so no species-level uniqueness is enforced.
func (s *UserGardenService) Add(userID uint, g *model.UserGarden) (*model.UserGarden, error) {
	g.UserID = userID
	if g.OwnedSince.IsZero() {
		g.OwnedSince = time.Now()
	}
	if err := s.repo.Create(g); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogGardenAddFailed, g.PlantSpeciesID, userID), "error", err)
		return nil, fmt.Errorf("user garden add: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogGardenAddSuccess, g.PlantSpeciesID, userID), "id", g.ID)
	return g, nil
}

// List returns a user's pots with the reminders bound to each one. Unbound
// reminders (legacy) are grouped under pot id 0.
func (s *UserGardenService) List(userID uint) ([]dto.GardenView, error) {
	pots, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("user garden list: %w", err)
	}
	views, err := s.reminderRepo.ListByUser(userID, "")
	if err != nil {
		return nil, fmt.Errorf("user garden list reminders: %w", err)
	}
	byPot := make(map[uint][]dto.ReminderView, len(pots))
	for _, v := range views {
		byPot[v.UserGardenID] = append(byPot[v.UserGardenID], v)
	}
	result := make([]dto.GardenView, 0, len(pots))
	for _, p := range pots {
		result = append(result, toGardenView(p, byPot[p.ID]))
	}
	if unbound := byPot[0]; len(unbound) > 0 {
		result = append(result, dto.GardenView{
			ID: 0, PlantSpeciesID: 0, Reminders: unbound,
		})
	}
	return result, nil
}

func toGardenView(p model.UserGarden, reminders []dto.ReminderView) dto.GardenView {
	if reminders == nil {
		reminders = []dto.ReminderView{}
	}
	return dto.GardenView{
		ID:             p.ID,
		UserID:         p.UserID,
		PlantSpeciesID: p.PlantSpeciesID,
		Nickname:       p.Nickname,
		OwnedSince:     p.OwnedSince,
		Location:       p.Location,
		CreatedAt:      p.CreatedAt,
		Reminders:      reminders,
	}
}

// Update edits the mutable nickname/location of a pot.
func (s *UserGardenService) Update(userID, id uint, nickname, location *string) (*model.UserGarden, error) {
	g, err := s.loadOwnedPot(userID, id)
	if err != nil {
		return nil, err
	}
	if nickname != nil {
		g.Nickname = *nickname
	}
	if location != nil {
		g.Location = *location
	}
	if err := s.repo.Update(g); err != nil {
		return nil, fmt.Errorf("user garden update: %w", err)
	}
	return g, nil
}

// Repot records a repotting: the pot's takeover date becomes ownedSince and
// every unfinished reminder bound to it is rescheduled relative to the new
// anchor date. Completed reminders are historical records and stay untouched.
// Pot update and reminder updates share one transaction, so a failure leaves
// neither side half-applied.
func (s *UserGardenService) Repot(userID, id uint, ownedSince time.Time, nickname, location *string) (*dto.GardenView, error) {
	if ownedSince.IsZero() {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("UserGarden[id=%d] repot failed: owned_since required", id))
	}
	newAnchor := dateOnly(ownedSince)

	var result dto.GardenView
	err := s.db.Transaction(func(tx *gorm.DB) error {
		pot, err := s.loadOwnedPot(userID, id)
		if err != nil {
			return err
		}
		oldAnchor := dateOnly(pot.OwnedSince)

		reminders, err := s.reminderRepo.ListByPotTx(tx, id)
		if err != nil {
			return fmt.Errorf("repot reminder list: %w", err)
		}
		for i := range reminders {
			r := &reminders[i]
			if r.Status == model.ReminderDone {
				continue
			}
			r.RemindDate = rescheduleDate(dateOnly(r.RemindDate), oldAnchor, newAnchor, r.Frequency)
			r.Status = statusForDate(r.RemindDate)
			if err := s.reminderRepo.UpdateTx(tx, r); err != nil {
				return fmt.Errorf("repot reminder update: %w", err)
			}
		}

		pot.OwnedSince = newAnchor
		if nickname != nil {
			pot.Nickname = *nickname
		}
		if location != nil {
			pot.Location = *location
		}
		if err := s.repo.UpdateTx(tx, pot); err != nil {
			return fmt.Errorf("repot pot update: %w", err)
		}

		updated, err := s.repo.FindByID(id)
		if err != nil {
			return fmt.Errorf("repot pot reload: %w", err)
		}
		potViews := make([]dto.ReminderView, 0, len(reminders))
		for _, r := range reminders {
			v, err := s.reminderRepo.GetViewByID(r.ID)
			if err != nil {
				return fmt.Errorf("repot reminder view: %w", err)
			}
			potViews = append(potViews, *v)
		}
		result = toGardenView(*updated, potViews)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("pot repotted, unfinished reminders rescheduled",
		"pot_id", id, "user_id", userID, "owned_since", newAnchor.Format("2006-01-02"))
	return &result, nil
}

// Remove deletes a pot owned by the user. Reminders bound to it are unbound
// (pot number cleared) rather than deleted, preserving their history. Both
// changes share one transaction.
func (s *UserGardenService) Remove(userID, id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.loadOwnedPot(userID, id); err != nil {
			return err
		}
		reminders, err := s.reminderRepo.ListByPotTx(tx, id)
		if err != nil {
			return fmt.Errorf("user garden remove reminders: %w", err)
		}
		for i := range reminders {
			reminders[i].UserGardenID = 0
			if err := s.reminderRepo.UpdateTx(tx, &reminders[i]); err != nil {
				return fmt.Errorf("user garden remove unbind: %w", err)
			}
		}
		if err := s.repo.DeleteTx(tx, id); err != nil {
			return fmt.Errorf("user garden remove: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.logger.Info("user garden item removed", "user_id", userID, "id", id)
	return nil
}

// BindReminder associates a care reminder with a pot, reusing the reminder
// service's transactional binding.
func (s *UserGardenService) BindReminder(userID, gardenID, reminderID uint) (*dto.ReminderView, error) {
	return NewCareReminderService(s.db, s.reminderRepo, s.repo, s.logger).BindToPot(userID, gardenID, reminderID)
}

func (s *UserGardenService) loadOwnedPot(userID, id uint) (*model.UserGarden, error) {
	g, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("UserGarden[id=%d] not found", id))
		}
		return nil, fmt.Errorf("user garden find: %w", err)
	}
	if g.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("UserGarden[id=%d] not owned by user_id=%d", id, userID))
	}
	return g, nil
}

// rescheduleDate shifts an unfinished reminder's date by the takeover-date
// delta (newAnchor - oldAnchor), then, if the result lands before the new
// anchor, rolls it forward by whole frequency steps so the first occurrence is
// not before the repotting day. Reminders without a frequency are only shifted.
func rescheduleDate(oldDate, oldAnchor, newAnchor time.Time, frequency string) time.Time {
	shifted := oldDate.AddDate(0, 0, calendarDays(newAnchor, oldAnchor))
	if !shifted.Before(newAnchor) {
		return shifted
	}
	for shifted.Before(newAnchor) {
		switch frequency {
		case model.FrequencyDaily:
			shifted = shifted.AddDate(0, 0, 1)
		case model.FrequencyWeekly:
			shifted = shifted.AddDate(0, 0, 7)
		case model.FrequencyMonthly:
			shifted = shifted.AddDate(0, 1, 0)
		case model.FrequencyYearly:
			shifted = shifted.AddDate(1, 0, 0)
		default:
			return shifted
		}
	}
	return shifted
}

// statusForDate recomputes pending/overdue for a rescheduled date.
func statusForDate(d time.Time) string {
	if d.Before(dateOnly(time.Now())) {
		return model.ReminderOverdue
	}
	return model.ReminderPending
}

func dateOnly(t time.Time) time.Time {
	t = t.Local()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// calendarDays returns the whole-day difference a - b, safe across months and
// DST transitions (both dates are normalized to UTC first).
func calendarDays(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	da := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	db := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(da.Sub(db).Hours() / 24)
}
