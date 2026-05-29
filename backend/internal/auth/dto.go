package auth

// --- login ---

type LoginRequest struct {
	Username string `json:"username" binding:"required,alphanum,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// --- register ---

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=32"`
	Key      string `json:"key"      binding:"required"`
}

type RegisterResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// --- token validation ---

type ValidateResponse struct {
	Valid    bool   `json:"valid"`
	UserID   uint   `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
}

// --- key management ---

type GenerateKeyResponse struct {
	Key string `json:"key"`
}

type KeyStatusResponse struct {
	Value    string `json:"value"`
	IsActive bool   `json:"is_active"`
}
