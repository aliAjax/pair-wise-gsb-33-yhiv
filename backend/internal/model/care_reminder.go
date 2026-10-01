package model

import "time"

// ReminderStatus values.
const (
	ReminderPending = "pending"
	ReminderDone    = "done"
	ReminderOverdue = "overdue"
)

// CareReminder is a scheduled gardening task owned by a user. A reminder is
// bound to one concrete pot (UserGarden via GardenID) instead of to a plant
// species, so reminders of divided plants never get mixed up.
type CareReminder struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"index;not null" json:"user_id"`
	GardenID uint `gorm:"index;not null;default:0;uniqueIndex:uk_reminder_plan,priority:1" json:"garden_id"`
	// PlantSpeciesID is denormalized from the pot for filtering/statistics and
	// for migrating legacy rows that had no pot binding.
	PlantSpeciesID uint      `gorm:"index" json:"plant_species_id"`
	TaskTitle      string    `gorm:"size:255;not null;uniqueIndex:uk_reminder_plan,priority:2" json:"task_title"`
	RemindDate     time.Time `gorm:"type:date;index;uniqueIndex:uk_reminder_plan,priority:3" json:"remind_date"`
	Frequency      string    `gorm:"size:32" json:"frequency"`
	Status         string    `gorm:"size:16;default:pending;index" json:"status"`
	CreatedAt      time.Time `json:"created_at"`

	// Enrichment fields (not table columns), populated from the bound pot and
	// its species so every view shows the same pot number / plant name.
	PotNo     string `gorm:"->;column:pot_no" json:"pot_no,omitempty"`
	PotStatus string `gorm:"->;column:pot_status" json:"pot_status,omitempty"`
	PlantName string `gorm:"->;column:plant_name" json:"plant_name,omitempty"`
}
