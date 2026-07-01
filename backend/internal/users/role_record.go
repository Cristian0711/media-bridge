package users

// RoleRecord is the roles lookup table (admin, user).
type RoleRecord struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"uniqueIndex;not null;size:32"`
}

func (RoleRecord) TableName() string {
	return "roles"
}
