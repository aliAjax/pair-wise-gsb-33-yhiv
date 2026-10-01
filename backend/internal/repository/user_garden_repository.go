package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/gbplantwiki/gbplantwiki/internal/model"
)

// UserGardenRepository handles persistence of user garden pots.
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

// CreateTx inserts a pot within an outer transaction.
func (r *UserGardenRepository) CreateTx(tx *gorm.DB, g *model.UserGarden) error {
	if err := tx.Create(g).Error; err != nil {
		if isDuplicate(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// Find locates a pot by user and plant species. When several pots of the same
// species exist, the earliest-taken-over (active) pot is returned.
func (r *UserGardenRepository) Find(userID, plantID uint) (*model.UserGarden, error) {
	var g model.UserGarden
	if err := r.db.Where("user_id = ? AND plant_species_id = ?", userID, plantID).
		Order("owned_since ASC, id ASC").First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &g, nil
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

// Update persists a pot.
func (r *UserGardenRepository) Update(g *model.UserGarden) error {
	return r.UpdateTx(r.db, g)
}

// UpdateTx persists a pot within an outer transaction.
func (r *UserGardenRepository) UpdateTx(tx *gorm.DB, g *model.UserGarden) error {
	return tx.Save(g).Error
}

// Delete removes a pot by id.
func (r *UserGardenRepository) Delete(id uint) error {
	return r.db.Delete(&model.UserGarden{}, id).Error
}

// DeleteTx removes a pot by id within an outer transaction.
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

// CountByUser returns the number of pots owned by a user (for pot numbering).
func (r *UserGardenRepository) CountByUser(tx *gorm.DB, userID uint) (int64, error) {
	var count int64
	if err := tx.Model(&model.UserGarden{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListByUser returns all pots of a user enriched with the species name.
// Active pots come first, then repotted (historical) pots.
func (r *UserGardenRepository) ListByUser(userID uint) ([]model.UserGarden, error) {
	var items []model.UserGarden
	err := r.db.Table("user_gardens AS ug").
		Select("ug.*, ps.name AS plant_name").
		Joins("LEFT JOIN plant_species ps ON ps.id = ug.plant_species_id").
		Where("ug.user_id = ?", userID).
		Order("CASE ug.status WHEN 'active' THEN 0 ELSE 1 END, ug.owned_since DESC, ug.id DESC").
		Scan(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
