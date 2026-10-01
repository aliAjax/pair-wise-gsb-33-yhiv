package dto

import "github.com/gbplantwiki/gbplantwiki/internal/util"

// ReminderCreateRequest is the payload for creating a care reminder. GardenID
// binds the reminder to one concrete pot; when omitted, the earliest pot of
// the given species is used.
type ReminderCreateRequest struct {
	GardenID       uint       `json:"garden_id"`
	PlantSpeciesID uint       `json:"plant_species_id"`
	TaskTitle      string     `json:"task_title" binding:"required,max=255"`
	RemindDate     *util.Date `json:"remind_date" binding:"required"`
	Frequency      string     `json:"frequency" binding:"omitempty,max=32"`
}

// ReminderStatusRequest carries the new status for a reminder.
type ReminderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}
