package dto

import "time"

// GardenAddRequest registers a new pot of a plant in the user's garden.
// The same species may be registered repeatedly (e.g. after division); each
// pot carries its own location and takeover date.
type GardenAddRequest struct {
	PlantSpeciesID uint      `json:"plant_species_id" binding:"required"`
	Nickname       string    `json:"nickname" binding:"omitempty,max=64"`
	OwnedSince     time.Time `json:"owned_since"`
	Location       string    `json:"location" binding:"omitempty,max=128"`
}

// GardenUpdateRequest edits mutable attributes of a pot without changing its
// takeover date.
type GardenUpdateRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,max=64"`
	Location *string `json:"location" binding:"omitempty,max=128"`
}

// GardenRepotRequest records a repotting: a new takeover date is required and
// unfinished reminders bound to the pot are rescheduled from it.
type GardenRepotRequest struct {
	OwnedSince time.Time `json:"owned_since" binding:"required"`
	Nickname   *string   `json:"nickname" binding:"omitempty,max=64"`
	Location   *string   `json:"location" binding:"omitempty,max=128"`
}

// GardenBindRequest binds a reminder to a garden item (pot).
type GardenBindRequest struct {
	ReminderID uint `json:"care_reminder_id" binding:"required"`
}

// GardenView is a pot together with the reminders bound to it, so that the
// garden page, calendar and reminder list all render the same pot-reminder
// relationship.
type GardenView struct {
	ID             uint           `json:"id"`
	UserID         uint           `json:"user_id"`
	PlantSpeciesID uint           `json:"plant_species_id"`
	Nickname       string         `json:"nickname"`
	OwnedSince     time.Time      `json:"owned_since"`
	Location       string         `json:"location"`
	CreatedAt      time.Time      `json:"created_at"`
	Reminders      []ReminderView `json:"reminders"`
}
