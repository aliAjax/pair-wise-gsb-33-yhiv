package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/dto"
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

// Create inserts a reminder. A duplicate of the same plan (same user, pot,
// title and date) surfaces as ErrDuplicate so the caller can return the
// already-submitted plan to late concurrent requests.
func (r *CareReminderRepository) Create(m *model.CareReminder) error {
	return r.CreateTx(r.db, m)
}

// CreateTx inserts a reminder inside an outer transaction.
func (r *CareReminderRepository) CreateTx(tx *gorm.DB, m *model.CareReminder) error {
	if err := tx.Create(m).Error; err != nil {
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// FindPlan locates an existing plan by its uniqueness tuple.
func (r *CareReminderRepository) FindPlan(userID, gardenID uint, title string, date time.Time) (*model.CareReminder, error) {
	return r.FindPlanTx(r.db, userID, gardenID, title, date)
}

// FindPlanTx locates an existing plan inside an outer transaction.
func (r *CareReminderRepository) FindPlanTx(tx *gorm.DB, userID, gardenID uint, title string, date time.Time) (*model.CareReminder, error) {
	var m model.CareReminder
	err := tx.Where("user_id = ? AND user_garden_id = ? AND task_title = ? AND remind_date = ?",
		userID, gardenID, title, date).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
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

// Update persists a reminder.
func (r *CareReminderRepository) Update(m *model.CareReminder) error {
	return r.UpdateTx(r.db, m)
}

// UpdateTx persists a reminder inside an outer transaction.
func (r *CareReminderRepository) UpdateTx(tx *gorm.DB, m *model.CareReminder) error {
	return tx.Save(m).Error
}

// Delete removes a reminder by id.
func (r *CareReminderRepository) Delete(id uint) error {
	return r.DeleteTx(r.db, id)
}

// DeleteTx removes a reminder by id inside an outer transaction.
func (r *CareReminderRepository) DeleteTx(tx *gorm.DB, id uint) error {
	res := tx.Delete(&model.CareReminder{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// reminderJoin is the SELECT shared by all read views. A LEFT JOIN keeps
// legacy reminders (user_garden_id = 0) visible, and COALESCE keeps pot display
// fields at zero values instead of NULL.
const reminderJoin = `care_reminders.id, care_reminders.user_id, care_reminders.user_garden_id,
	care_reminders.plant_species_id, care_reminders.task_title, care_reminders.remind_date,
	care_reminders.frequency, care_reminders.status, care_reminders.created_at,
	COALESCE(user_gardens.id, 0) AS pot_number,
	COALESCE(user_gardens.nickname, '') AS pot_nickname,
	COALESCE(user_gardens.location, '') AS pot_location
FROM care_reminders
LEFT JOIN user_gardens ON user_gardens.id = care_reminders.user_garden_id`

// scanViews runs a reminder query and maps rows to ReminderView.
func scanViews(tx *gorm.DB, where string, args []any, order string) ([]dto.ReminderView, error) {
	var views []dto.ReminderView
	q := tx.Table("care_reminders").
		Select(reminderJoin).
		Where(where, args...)
	if order != "" {
		q = q.Order(order)
	}
	if err := q.Scan(&views).Error; err != nil {
		return nil, err
	}
	return views, nil
}

// ListByUser returns reminders for a user with optional status filter.
func (r *CareReminderRepository) ListByUser(userID uint, status string) ([]dto.ReminderView, error) {
	where := "care_reminders.user_id = ?"
	args := []any{userID}
	if status != "" {
		where += " AND care_reminders.status = ?"
		args = append(args, status)
	}
	return scanViews(r.db, where, args, "care_reminders.remind_date ASC")
}

// GetViewByID returns one reminder with its pot display fields.
func (r *CareReminderRepository) GetViewByID(id uint) (*dto.ReminderView, error) {
	views, err := scanViews(r.db, "care_reminders.id = ?", []any{id}, "")
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, ErrNotFound
	}
	return &views[0], nil
}

// ListByMonth returns reminders for a user within a month of a given year.
func (r *CareReminderRepository) ListByMonth(userID uint, year, month int) ([]dto.ReminderView, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0)
	return scanViews(r.db,
		"care_reminders.user_id = ? AND care_reminders.remind_date >= ? AND care_reminders.remind_date < ?",
		[]any{userID, start, end}, "care_reminders.remind_date ASC")
}

// ListByPotTx returns every reminder bound to a pot inside a transaction.
func (r *CareReminderRepository) ListByPotTx(tx *gorm.DB, gardenID uint) ([]model.CareReminder, error) {
	var items []model.CareReminder
	if err := tx.Where("user_garden_id = ?", gardenID).
		Order("remind_date ASC, id ASC").Find(&items).Error; err != nil {
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

// AssignLegacyReminders assigns reminders without a pot number to the pot of
// the same species that the user took over first (smallest id breaks ties).
// Reminders whose species has no pot at all stay unassigned (0).
func (r *CareReminderRepository) AssignLegacyReminders() error {
	return r.db.Exec(`
		UPDATE care_reminders r
		SET r.user_garden_id = (
			SELECT g.id FROM user_gardens g
			WHERE g.user_id = r.user_id AND g.plant_species_id = r.plant_species_id
			ORDER BY g.owned_since ASC, g.id ASC
			LIMIT 1
		)
		WHERE r.user_garden_id = 0 AND r.plant_species_id > 0
		  AND EXISTS (
			SELECT 1 FROM user_gardens g2
			WHERE g2.user_id = r.user_id AND g2.plant_species_id = r.plant_species_id
		)`).Error
}
