package models

type ShortenURLRequest struct {
	URL              string            `json:"url" binding:"required,url"`
	ExpirationPreset string            `json:"expiration_preset,omitempty"` // "1hour" | "12hours" | "1day" | "7days" | "1month" | "6months" | "1year" | "default"
	CustomExpiration *CustomExpiration `json:"custom_expiration,omitempty"`
}

type CustomExpiration struct {
	Years   string `json:"years"`   // "0" | "1" | "2" | "3" | "4"
	Months  string `json:"months"`  // "0" | "1" | ... | "11"
	Days    string `json:"days"`    // "0" | "1" | ... | "30"
	Hours   string `json:"hours"`   // "0" | "1" | ... | "23"
	Minutes string `json:"minutes"` // "0" | "1" | ... | "59"
}

type UpdateURLRequest struct {
	URL              string            `json:"url,omitempty"`               // Optional: only if URL changed
	ExpirationPreset string            `json:"expiration_preset,omitempty"` // Optional: only if expiration changed
	CustomExpiration *CustomExpiration `json:"custom_expiration,omitempty"` // Optional: only if custom expiration selected
}

type UpdateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active paused"` // Required: must be "active" or "paused"
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
	User  *User  `json:"user"`
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
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    *User      `json:"data,omitempty"`
	Error   *AuthError `json:"error,omitempty"`
}

// Support Request DTOs
type SupportRequest struct {
	Name    string `json:"name" binding:"required,min=1,max=100"`
	Email   string `json:"email" binding:"required,email,max=255"`
	Message string `json:"message" binding:"required,min=1,max=5000"`
}

type SupportResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Error   *AuthError `json:"error,omitempty"`
}
