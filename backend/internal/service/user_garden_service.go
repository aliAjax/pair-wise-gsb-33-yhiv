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

// UserGardenService implements "my garden" logic at pot granularity: one
// species may live in several pots, every pot registers its own location and
// takeover date.
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

// nextPotNo allocates the next sequential pot number for a user, based on the
// number of already registered pots.
func nextPotNo(existing int64) string {
	return fmt.Sprintf("P%04d", existing+1)
}

// repotDeltaDays is the whole-day shift applied to open reminders when
// repotting: the new takeover date becomes the new anchor ("新起算日"), and
// every pending/overdue reminder moves by exactly that many days.
func repotDeltaDays(oldOwnedSince, newOwnedSince time.Time) int {
	oldDay := time.Date(oldOwnedSince.Year(), oldOwnedSince.Month(), oldOwnedSince.Day(), 0, 0, 0, 0, time.UTC)
	newDay := time.Date(newOwnedSince.Year(), newOwnedSince.Month(), newOwnedSince.Day(), 0, 0, 0, 0, time.UTC)
	return int(newDay.Sub(oldDay).Hours() / 24)
}

// Add registers a new pot independently, even when another pot of the same
// species exists (division produces multiple pots).
func (s *UserGardenService) Add(userID uint, g *model.UserGarden) (*model.UserGarden, error) {
	g.UserID = userID
	g.Status = model.PotActive
	if g.OwnedSince.IsZero() {
		g.OwnedSince = time.Now()
	}
	for attempt := 0; attempt < uniqueRaceRetries; attempt++ {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			// Pot numbering is count+1. A concurrent add makes numbering
			// collide on uk_garden_pot_no; restart with a fresh snapshot
			// instead of retrying inside the same stale read.
			count, err := s.repo.CountByUser(tx, userID)
			if err != nil {
				return fmt.Errorf("user garden pot count: %w", err)
			}
			g.PotNo = nextPotNo(count)
			if err := s.repo.CreateTx(tx, g); err != nil {
				if errors.Is(err, repository.ErrDuplicate) {
					return errUniqueRace
				}
				s.logger.Error(fmt.Sprintf(constants.LogGardenAddFailed, g.PlantSpeciesID, userID), "error", err)
				return fmt.Errorf("user garden add: %w", err)
			}
			return nil
		})
		if err == nil {
			s.logger.Info(fmt.Sprintf(constants.LogGardenAddSuccess, g.PlantSpeciesID, userID),
				"id", g.ID, "pot_no", g.PotNo)
			return g, nil
		}
		if !errors.Is(err, errUniqueRace) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("user garden add: pot number allocation exhausted")
}

// List returns a user's pots enriched with the species name.
func (s *UserGardenService) List(userID uint) ([]model.UserGarden, error) {
	items, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("user garden list: %w", err)
	}
	return items, nil
}

// Get loads a pot and verifies ownership.
func (s *UserGardenService) Get(userID, id uint) (*model.UserGarden, error) {
	g, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("UserGarden[id=%d] not found", id))
		}
		return nil, fmt.Errorf("user garden get: %w", err)
	}
	if g.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("UserGarden[id=%d] access failed: not owner", id))
	}
	return g, nil
}

// Update edits the registered information of a pot (nickname / location /
// takeover date) without touching any reminders.
func (s *UserGardenService) Update(userID, id uint, nickname, location string, ownedSince *time.Time) (*model.UserGarden, error) {
	g, err := s.Get(userID, id)
	if err != nil {
		return nil, err
	}
	g.Nickname = nickname
	g.Location = location
	if ownedSince != nil {
		g.OwnedSince = *ownedSince
	}
	if err := s.repo.Update(g); err != nil {
		return nil, fmt.Errorf("user garden update: %w", err)
	}
	return g, nil
}

// Repot moves a plant from an old pot into a freshly registered pot:
//   - a new pot row is created with the new takeover date and location;
//   - not-yet-done reminders are rebound to the new pot and shifted by the
//     delta between new and old takeover date (重排到新起算日);
//   - done reminders stay on the old pot with their original dates
//     (已完成记录保持原样);
//   - the old pot is marked repotted as history.
//
// Pot write and reminder writes share one transaction: failure rolls both
// back, never leaving half the data on either side.
func (s *UserGardenService) Repot(userID, oldID uint, nickname, location string, newOwnedSince time.Time) (*model.UserGarden, error) {
	old, err := s.Get(userID, oldID)
	if err != nil {
		return nil, err
	}
	if old.Status != model.PotActive {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("UserGarden[id=%d] repot failed: pot already repotted", oldID))
	}
	if newOwnedSince.IsZero() {
		newOwnedSince = time.Now()
	}
	if !newOwnedSince.After(old.OwnedSince) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("UserGarden[id=%d] repot failed: new takeover date must be after %s", oldID, old.OwnedSince.Format("2006-01-02")))
	}
	if nickname == "" {
		nickname = old.Nickname
	}

	deltaDays := repotDeltaDays(old.OwnedSince, newOwnedSince)
	newPot := &model.UserGarden{
		UserID:         userID,
		PlantSpeciesID: old.PlantSpeciesID,
		Nickname:       nickname,
		OwnedSince:     newOwnedSince,
		Location:       location,
		Status:         model.PotActive,
	}

	var txErr error
	for attempt := 0; attempt < uniqueRaceRetries; attempt++ {
		newPot.ID = 0
		txErr = s.db.Transaction(func(tx *gorm.DB) error {
			var loadedOld model.UserGarden
			if err := tx.First(&loadedOld, oldID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("UserGarden[id=%d] not found", oldID))
				}
				return fmt.Errorf("repot old pot load: %w", err)
			}
			if loadedOld.UserID != userID {
				return util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("UserGarden[id=%d] repot failed: not owner", oldID))
			}
			if loadedOld.Status != model.PotActive {
				return util.NewAppError(409, constants.CodeConflict,
					fmt.Sprintf("UserGarden[id=%d] repot failed: pot already repotted", oldID))
			}
			count, err := s.repo.CountByUser(tx, userID)
			if err != nil {
				return fmt.Errorf("repot pot count: %w", err)
			}
			newPot.PotNo = nextPotNo(count)
			if err := s.repo.CreateTx(tx, newPot); err != nil {
				if errors.Is(err, repository.ErrDuplicate) {
					// Concurrent pot registration grabbed this pot number;
					// restart the whole repot with a fresh snapshot.
					return errUniqueRace
				}
				return fmt.Errorf("repot new pot create: %w", err)
			}
			if err := s.reminderRepo.RescheduleOpenByGardenTx(tx, old.ID, newPot.ID, deltaDays); err != nil {
				return fmt.Errorf("repot reminder reschedule: %w", err)
			}
			old.Status = model.PotRepotted
			if err := s.repo.UpdateTx(tx, old); err != nil {
				return fmt.Errorf("repot old pot update: %w", err)
			}
			return nil
		})
		if txErr == nil {
			s.logger.Info(fmt.Sprintf(constants.LogGardenRepotSuccess, oldID, newPot.ID),
				"new_pot_no", newPot.PotNo, "delta_days", deltaDays)
			return newPot, nil
		}
		if !errors.Is(txErr, errUniqueRace) {
			return nil, txErr
		}
	}
	return nil, fmt.Errorf("repot new pot create: pot number allocation exhausted")
}

// Remove deletes a pot together with ALL its reminders in one transaction, so
// a failure cannot leave a pot without reminders or reminders without a pot.
func (s *UserGardenService) Remove(userID, id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		g, err := s.getForUpdate(tx, userID, id)
		if err != nil {
			return err
		}
		if err := s.reminderRepo.DeleteByGardenTx(tx, g.ID); err != nil {
			return fmt.Errorf("user garden remove reminders: %w", err)
		}
		if err := s.repo.DeleteTx(tx, g.ID); err != nil {
			return fmt.Errorf("user garden remove pot: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.logger.Info(fmt.Sprintf(constants.LogGardenRemoveWithReminders, id, userID))
	return nil
}

// getForUpdate loads a pot inside a transaction and verifies ownership.
func (s *UserGardenService) getForUpdate(tx *gorm.DB, userID, id uint) (*model.UserGarden, error) {
	var g model.UserGarden
	if err := tx.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("UserGarden[id=%d] not found", id))
		}
		return nil, fmt.Errorf("user garden load: %w", err)
	}
	if g.UserID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden, fmt.Sprintf("UserGarden[id=%d] access failed: not owner", id))
	}
	return &g, nil
}

// BindReminder binds an existing reminder to a pot. Both must belong to the
// caller; the denormalized species is synced from the pot. Pot and reminder
// are updated in one transaction.
func (s *UserGardenService) BindReminder(userID, gardenID, reminderID uint) (*model.UserGarden, error) {
	pot, err := s.Get(userID, gardenID)
	if err != nil {
		return nil, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var m model.CareReminder
		if err := tx.First(&m, reminderID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("CareReminder[id=%d] not found", reminderID))
			}
			return fmt.Errorf("user garden bind reminder find: %w", err)
		}
		if m.UserID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("CareReminder[id=%d] bind failed: not owner", reminderID))
		}
		m.GardenID = pot.ID
		m.PlantSpeciesID = pot.PlantSpeciesID
		if err := tx.Save(&m).Error; err != nil {
			return fmt.Errorf("user garden bind reminder update: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pot, nil
}
