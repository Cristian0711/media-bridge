package users

// CreateInput is used internally by the auth service during registration.
type CreateInput struct {
	Username     string
	PasswordHash string
}

type UserResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}