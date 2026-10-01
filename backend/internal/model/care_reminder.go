package model

import "time"

// ReminderStatus values.
const (
	ReminderPending = "pending"
	ReminderDone    = "done"
	ReminderOverdue = "overdue"
)

// ReminderFrequency values accepted when creating a reminder.
const (
	FrequencyNone    = ""
	FrequencyDaily   = "daily"
	FrequencyWeekly  = "weekly"
	FrequencyMonthly = "monthly"
	FrequencyYearly  = "yearly"
)

// CareReminder is a scheduled gardening task owned by a user. A reminder is
// bound to a concrete pot (UserGardenID); 0 means a legacy reminder that was
// created before reminders carried a pot number.
type CareReminder struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"uniqueIndex:uk_reminder_plan,priority:1;not null" json:"user_id"`
	UserGardenID   uint      `gorm:"column:user_garden_id;not null;default:0;uniqueIndex:uk_reminder_plan,priority:2;index" json:"user_garden_id"`
	PlantSpeciesID uint      `gorm:"index" json:"plant_species_id"`
	TaskTitle      string    `gorm:"size:255;not null;uniqueIndex:uk_reminder_plan,priority:3" json:"task_title"`
	RemindDate     time.Time `gorm:"type:date;index;uniqueIndex:uk_reminder_plan,priority:4" json:"remind_date"`
	Frequency      string    `gorm:"size:32" json:"frequency"`
	Status         string    `gorm:"size:16;default:pending;index" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}
