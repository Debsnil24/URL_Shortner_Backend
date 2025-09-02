package service

import (
	"errors"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

func (s *AuthService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	var existing models.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else if existing.ID != [16]byte{} { // non-zero UUID means found
		return &models.AuthResponse{
			Success: false,
			Error:   &models.AuthError{Code: "AUTH_409", Message: "Email already in use"},
		}, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Email:         email,
		PasswordHash:  string(hash),
		Provider:      "email",
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		EmailVerified: false,
		IsActive:      true,
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	token, err := util.GenerateToken(user.ID.String(), user.Email, user.Provider)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Success: true,
		Message: "User registered successfully",
		Data:    &models.AuthData{User: &user, Token: token},
	}, nil
}

func (s *AuthService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Invalid credentials"}}, nil
		}
		return nil, err
	}

	if user.Provider != "email" {
		return &models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_400", Message: "Account uses OAuth provider"}}, nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return &models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Invalid credentials"}}, nil
	}

	now := time.Now()
	user.LastLogin = &now
	_ = s.db.Model(&user).Update("last_login", now).Error

	token, err := util.GenerateToken(user.ID.String(), user.Email, user.Provider)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Success: true,
		Message: "Login successful",
		Data:    &models.AuthData{User: &user, Token: token},
	}, nil
}
