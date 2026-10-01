package model

import "time"

// Pot status values.
const (
	PotActive   = "active"   // 在养花盆
	PotRepotted = "repotted" // 已换盆，原盆保留作为历史记录
)

// UserGarden represents one physical pot owned by a user inside their garden.
// The same plant species may be grown in multiple pots (e.g. after division);
// each pot registers its own location and takeover (owned_since) date.
type UserGarden struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	UserID         uint `gorm:"index:idx_garden_user;uniqueIndex:uk_garden_pot_no,priority:1;not null" json:"user_id"`
	PlantSpeciesID uint `gorm:"index:idx_garden_user;not null" json:"plant_species_id"`
	// PotNo is the human-facing pot number, unique per user and restarting at
	// P0001 for every user.
	PotNo    string `gorm:"size:32;uniqueIndex:uk_garden_pot_no,priority:2;not null" json:"pot_no"`
	Nickname string `gorm:"size:64" json:"nickname"`
	// OwnedSince is the takeover date of THIS pot. Repotting starts a new pot
	// with a new OwnedSince and leaves the old pot record untouched.
	OwnedSince time.Time `gorm:"type:date" json:"owned_since"`
	Location   string    `gorm:"size:128" json:"location"`
	Status     string    `gorm:"size:16;default:active;index;not null" json:"status"`
	CreatedAt  time.Time `json:"created_at"`

	// Enrichment fields (not a table column), populated from plant_species so
	// that the garden page, calendar and reminder list render the same names.
	PlantName string `gorm:"->;column:plant_name" json:"plant_name,omitempty"`
}
