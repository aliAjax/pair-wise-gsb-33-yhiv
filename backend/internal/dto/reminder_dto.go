package dto

import "time"

// ReminderCreateRequest is the payload for creating a care reminder.
// UserGardenID binds the plan to a concrete pot; it is required for new plans.
type ReminderCreateRequest struct {
	UserGardenID   uint      `json:"user_garden_id"`
	PlantSpeciesID uint      `json:"plant_species_id"`
	TaskTitle      string    `json:"task_title" binding:"required,max=255"`
	RemindDate     time.Time `json:"remind_date" binding:"required"`
	Frequency      string    `json:"frequency" binding:"omitempty,max=32"`
}

// ReminderStatusRequest carries the new status for a reminder.
type ReminderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// ReminderView is a reminder enriched with the bound pot's display data. Every
// read surface (reminder list, calendar month, garden list) returns this shape
// so the pot number shown is identical everywhere.
type ReminderView struct {
	ID             uint      `json:"id"`
	UserID         uint      `json:"user_id"`
	UserGardenID   uint      `json:"user_garden_id"`
	PotNumber      uint      `json:"pot_number"`
	PlantSpeciesID uint      `json:"plant_species_id"`
	PotNickname    string    `json:"pot_nickname"`
	PotLocation    string    `json:"pot_location"`
	TaskTitle      string    `json:"task_title"`
	RemindDate     time.Time `json:"remind_date"`
	Frequency      string    `json:"frequency"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}
