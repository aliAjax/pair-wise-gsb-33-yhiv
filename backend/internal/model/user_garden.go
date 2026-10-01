package model

import "time"

// UserGarden represents one individual pot of a plant owned by a user.
// The same plant species may be registered several times (e.g. after division),
// therefore each row is an independent pot with its own location and the date
// the user took it over (OwnedSince).
type UserGarden struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"index:idx_garden_user;not null" json:"user_id"`
	PlantSpeciesID uint      `gorm:"index:idx_garden_user_plant;not null" json:"plant_species_id"`
	Nickname       string    `gorm:"size:64" json:"nickname"`
	OwnedSince     time.Time `gorm:"type:date" json:"owned_since"`
	Location       string    `gorm:"size:128" json:"location"`
	CreatedAt      time.Time `json:"created_at"`
}
