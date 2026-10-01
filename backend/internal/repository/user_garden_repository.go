package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// UserGardenRepository handles persistence of user garden items (pots).
type UserGardenRepository struct {
	db *gorm.DB
}

// NewUserGardenRepository creates a UserGardenRepository.
func NewUserGardenRepository(db *gorm.DB) *UserGardenRepository {
	return &UserGardenRepository{db: db}
}

// Create inserts a pot.
func (r *UserGardenRepository) Create(g *model.UserGarden) error {
	return r.CreateTx(r.db, g)
}

// CreateTx inserts a pot inside an outer transaction.
func (r *UserGardenRepository) CreateTx(tx *gorm.DB, g *model.UserGarden) error {
	return tx.Create(g).Error
}

// FindByID locates a pot by primary key.
func (r *UserGardenRepository) FindByID(id uint) (*model.UserGarden, error) {
	var g model.UserGarden
	if err := r.db.First(&g, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// FindEarliestPot returns the pot of a species the user took over first.
// Ties are broken by the smallest id. Used to assign legacy reminders that
// predate per-pot binding.
func (r *UserGardenRepository) FindEarliestPot(userID, plantID uint) (*model.UserGarden, error) {
	var g model.UserGarden
	err := r.db.Where("user_id = ? AND plant_species_id = ?", userID, plantID).
		Order("owned_since ASC, id ASC").First(&g).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

// Update persists a pot.
func (r *UserGardenRepository) Update(g *model.UserGarden) error {
	return r.UpdateTx(r.db, g)
}

// UpdateTx persists a pot inside an outer transaction.
func (r *UserGardenRepository) UpdateTx(tx *gorm.DB, g *model.UserGarden) error {
	return tx.Save(g).Error
}

// Delete removes a pot by id.
func (r *UserGardenRepository) Delete(id uint) error {
	return r.DeleteTx(r.db, id)
}

// DeleteTx removes a pot by id inside an outer transaction.
func (r *UserGardenRepository) DeleteTx(tx *gorm.DB, id uint) error {
	res := tx.Delete(&model.UserGarden{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser returns all pots of a user, oldest pot of each species first.
func (r *UserGardenRepository) ListByUser(userID uint) ([]model.UserGarden, error) {
	var items []model.UserGarden
	if err := r.db.Where("user_id = ?", userID).
		Order("plant_species_id ASC, owned_since ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
