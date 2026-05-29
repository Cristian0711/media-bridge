package users

import (
	"errors"

	"gorm.io/gorm"
)

// SeedRuntime inserts default roles and ensures an admin exists when users are present.
func SeedRuntime(db *gorm.DB) error {
	if err := seedRoles(db); err != nil {
		return err
	}
	return ensureFirstUserIsAdmin(db)
}

func seedRoles(db *gorm.DB) error {
	for _, name := range []string{RoleAdmin, RoleUser} {
		var role RoleRecord
		err := db.Where("name = ?", name).FirstOrCreate(&role, RoleRecord{Name: name}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureFirstUserIsAdmin promotes the earliest user when no admin exists yet.
func ensureFirstUserIsAdmin(db *gorm.DB) error {
	var adminCount int64
	if err := db.Model(&User{}).Where("role = ?", RoleAdmin).Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount > 0 {
		return nil
	}

	var first User
	err := db.Order("id ASC").First(&first).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return db.Model(&first).Update("role", RoleAdmin).Error
}
