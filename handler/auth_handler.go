package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type AuthHandler struct {
	svc *controller.AuthService
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{svc: controller.NewAuthService(db)}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "VALIDATION_001", Message: "Validation failed", Details: err.Error()}})
		return
	}

	// Basic password policy check (length already validated by binding)
	if len(strings.TrimSpace(req.Password)) < 8 {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "VALIDATION_002", Message: "Password too short"}})
		return
	}

	resp, err := h.svc.Register(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_500", Message: "Failed to register", Details: err.Error()}})
		return
	}

	// Conflict or validation returned by service
	if !resp.Success && resp.Error != nil && resp.Error.Code == "AUTH_409" {
		c.JSON(http.StatusConflict, resp)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "VALIDATION_001", Message: "Validation failed", Details: err.Error()}})
		return
	}

	resp, err := h.svc.Login(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_500", Message: "Failed to login", Details: err.Error()}})
		return
	}

	if !resp.Success && resp.Error != nil && (resp.Error.Code == "AUTH_401" || resp.Error.Code == "AUTH_400") {
		c.JSON(http.StatusUnauthorized, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Google OAuth
func (h *AuthHandler) GoogleAuth(c *gin.Context) {
	state := "state" // TODO: random + secure storage
	url := config.GoogleOAuthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("access_type", "offline"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_001", Message: "Missing authorization code"}})
		return
	}

	token, err := config.GoogleOAuthConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_002", Message: "Code exchange failed", Details: err.Error()}})
		return
	}

	client := config.GoogleOAuthConfig.Client(c, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil || resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_003", Message: "Failed to fetch user profile"}})
		return
	}
	defer resp.Body.Close()

	var gu struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_004", Message: "Invalid profile response"}})
		return
	}

	if gu.Email == "" || !gu.EmailVerified {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_005", Message: "Email not verified with Google"}})
		return
	}

	email := strings.ToLower(gu.Email)
	var user models.User
	if err := h.svc.DB().Where("email = ?", email).First(&user).Error; err == nil {
		user.Provider = "google"
		user.ProviderID = gu.Sub
		user.FirstName = gu.GivenName
		user.LastName = gu.FamilyName
		user.AvatarURL = gu.Picture
		user.EmailVerified = true
		now := time.Now()
		user.LastLogin = &now
		_ = h.svc.DB().Save(&user).Error
	} else {
		user = models.User{
			Email:         email,
			Provider:      "google",
			ProviderID:    gu.Sub,
			FirstName:     gu.GivenName,
			LastName:      gu.FamilyName,
			AvatarURL:     gu.Picture,
			EmailVerified: true,
			IsActive:      true,
		}
		if err := h.svc.DB().Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "OAUTH_006", Message: "Failed to create user", Details: err.Error()}})
			return
		}
	}

	jwt, err := util.GenerateToken(user.ID.String(), user.Email, user.Provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_500", Message: "Failed to issue token"}})
		return
	}

	fe := os.Getenv("FRONTEND_URL")
	if fe != "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/auth/callback?token=%s", fe, jwt))
		return
	}
	c.JSON(http.StatusOK, models.AuthResponse{Success: true, Message: "Google authentication successful", Data: &models.AuthData{User: &user, Token: jwt}})
}
