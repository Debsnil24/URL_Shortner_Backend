package models

type ShortenURLRequest struct {
	URL string `json:"url" binding:"required,url"`
}

type ShortenURLResponse struct {
	ShortenedURL string `json:"shortened_url"`
	OriginalURL  string `json:"original_url"`
	ShortCode    string `json:"short_code"`
}


// Authentication Request DTOs
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// Authentication Response DTOs
type AuthResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    *AuthData  `json:"data,omitempty"`
	Error   *AuthError `json:"error,omitempty"`
}

type AuthData struct {
	User  *User `json:"user"`
	Token string `json:"token"`
}

type AuthError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// User Profile DTOs
type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}

type UserProfileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    *User  `json:"data,omitempty"`
	Error   *AuthError `json:"error,omitempty"`
}