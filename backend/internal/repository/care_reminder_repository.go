package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// CareReminderRepository handles persistence of care reminders.
type CareReminderRepository struct {
	db *gorm.DB
}

// NewCareReminderRepository creates a CareReminderRepository.
func NewCareReminderRepository(db *gorm.DB) *CareReminderRepository {
	return &CareReminderRepository{db: db}
}

// enrichedSelect joins the bound pot and its species so every read view
// (garden, calendar, reminder list) renders identical pot/plant labels.
const enrichedSelect = "cr.*, ug.pot_no AS pot_no, ug.status AS pot_status, ps.name AS plant_name"

func enrichedQuery(db *gorm.DB) *gorm.DB {
	return db.Table("care_reminders AS cr").
		Select(enrichedSelect).
		Joins("LEFT JOIN user_gardens ug ON ug.id = cr.garden_id").
		Joins("LEFT JOIN plant_species ps ON ps.id = cr.plant_species_id")
}

// Create inserts a reminder.
func (r *CareReminderRepository) Create(m *model.CareReminder) error {
	return r.CreateTx(r.db, m)
}

// CreateTx inserts a reminder within an outer transaction.
func (r *CareReminderRepository) CreateTx(tx *gorm.DB, m *model.CareReminder) error {
	if err := tx.Create(m).Error; err != nil {
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// FindByID locates a reminder by id.
func (r *CareReminderRepository) FindByID(id uint) (*model.CareReminder, error) {
	var m model.CareReminder
	if err := r.db.First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// FindPlan locates an identical plan: same owner, same pot, same task and
// same due date. Used to return the existing plan when two devices submit
// the same plan concurrently (the unique index uk_reminder_plan is the
// race-safe backstop).
func (r *CareReminderRepository) FindPlan(tx *gorm.DB, userID, gardenID uint, taskTitle string, remindDate time.Time) (*model.CareReminder, error) {
	var m model.CareReminder
	day := remindDate.Format("2006-01-02")
	if err := tx.Where("user_id = ? AND garden_id = ? AND task_title = ? AND remind_date = ?",
		userID, gardenID, taskTitle, day).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Update persists a reminder.
func (r *CareReminderRepository) Update(m *model.CareReminder) error {
	return r.db.Save(m).Error
}

// Delete removes a reminder.
func (r *CareReminderRepository) Delete(id uint) error {
	res := r.db.Delete(&model.CareReminder{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByGardenTx removes every reminder of a pot within an outer transaction
// (used when deleting a pot, so pot and reminders never end up half-written).
func (r *CareReminderRepository) DeleteByGardenTx(tx *gorm.DB, gardenID uint) error {
	return tx.Where("garden_id = ?", gardenID).Delete(&model.CareReminder{}).Error
}

// ListByUser returns reminders for a user with optional status filter,
// enriched with pot number and plant name.
func (r *CareReminderRepository) ListByUser(userID uint, status string) ([]model.CareReminder, error) {
	var items []model.CareReminder
	q := enrichedQuery(r.db).Where("cr.user_id = ?", userID)
	if status != "" {
		q = q.Where("cr.status = ?", status)
	}
	if err := q.Order("cr.remind_date ASC, cr.id ASC").Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByMonth returns reminders for a user within a month of a given year.
func (r *CareReminderRepository) ListByMonth(userID uint, year, month int) ([]model.CareReminder, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	var items []model.CareReminder
	if err := enrichedQuery(r.db).
		Where("cr.user_id = ? AND cr.remind_date >= ? AND cr.remind_date < ?", userID, start, end).
		Order("cr.remind_date ASC, cr.id ASC").Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// MarkOverdue flips pending reminders whose date has passed to overdue.
func (r *CareReminderRepository) MarkOverdue(userID uint) (int64, error) {
	res := r.db.Model(&model.CareReminder{}).
		Where("user_id = ? AND status = ? AND remind_date < ?", userID, model.ReminderPending, time.Now()).
		Update("status", model.ReminderOverdue)
	return res.RowsAffected, res.Error
}

// ListUnboundByUser returns legacy reminders of a user that are not bound to
// any pot (garden_id = 0) but do reference a species.
func (r *CareReminderRepository) ListUnboundByUser(tx *gorm.DB, userID uint) ([]model.CareReminder, error) {
	var items []model.CareReminder
	if err := tx.Where("user_id = ? AND garden_id = 0 AND plant_species_id > 0", userID).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// EarliestPotOfSpecies returns the earliest-taken-over pot of a species for a
// user. Legacy reminders without a pot number are attributed to that pot.
func (r *CareReminderRepository) EarliestPotOfSpecies(tx *gorm.DB, userID, plantSpeciesID uint) (*model.UserGarden, error) {
	var g model.UserGarden
	if err := tx.Where("user_id = ? AND plant_species_id = ?", userID, plantSpeciesID).
		Order("owned_since ASC, id ASC").First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// BindToGardenTx sets the pot of a reminder within an outer transaction.
func (r *CareReminderRepository) BindToGardenTx(tx *gorm.DB, reminderID, gardenID uint) error {
	return tx.Model(&model.CareReminder{}).Where("id = ?", reminderID).
		Update("garden_id", gardenID).Error
}

// RescheduleOpenByGardenTx shifts every not-done reminder of a pot by the
// delta between the new and old takeover date. Done reminders are deliberately
// left untouched: completed history stays as recorded.
func (r *CareReminderRepository) RescheduleOpenByGardenTx(tx *gorm.DB, oldGardenID uint, newGardenID uint, deltaDays int) error {
	return tx.Model(&model.CareReminder{}).
		Where("garden_id = ? AND status <> ?", oldGardenID, model.ReminderDone).
		Updates(map[string]interface{}{
			"garden_id":   newGardenID,
			"remind_date": gorm.Expr("DATE_ADD(remind_date, INTERVAL ? DAY)", deltaDays),
		}).Error
}
