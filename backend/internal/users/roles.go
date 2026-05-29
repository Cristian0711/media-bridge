package users

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

func IsAdmin(role string) bool {
	return role == RoleAdmin
}
