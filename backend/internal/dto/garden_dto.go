package dto

import "github.com/gbplantwiki/gbplantwiki/internal/util"

// GardenAddRequest registers an individual pot in the user's garden.
type GardenAddRequest struct {
	PlantSpeciesID uint       `json:"plant_species_id" binding:"required"`
	Nickname       string     `json:"nickname" binding:"omitempty,max=64"`
	OwnedSince     *util.Date `json:"owned_since"`
	Location       string     `json:"location" binding:"omitempty,max=128"`
}

// GardenUpdateRequest edits the registration of an existing pot.
type GardenUpdateRequest struct {
	Nickname   *string    `json:"nickname" binding:"omitempty,max=64"`
	Location   *string    `json:"location" binding:"omitempty,max=128"`
	OwnedSince *util.Date `json:"owned_since"`
}

// GardenRepotRequest moves the plant into a new pot; the new takeover date is
// the new anchor ("新起算日") for open reminders.
type GardenRepotRequest struct {
	Nickname   string     `json:"nickname" binding:"omitempty,max=64"`
	Location   string     `json:"location" binding:"omitempty,max=128"`
	OwnedSince *util.Date `json:"owned_since" binding:"required"`
}

// GardenBindRequest binds a reminder to a concrete pot.
type GardenBindRequest struct {
	ReminderID uint `json:"care_reminder_id" binding:"required"`
}
