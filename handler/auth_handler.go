package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Debsnil24/URL_Shortner.git/config"
	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type AuthHandler struct {
	svc *controller.AuthService
}

// generateRandomState generates a cryptographically secure random state string
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
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
	// Generate a secure random state
	state, err := generateRandomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.AuthResponse{
			Success: false,
			Error:   &models.AuthError{Code: "OAUTH_000", Message: "Failed to generate state parameter"},
		})
		return
	}

	// Store state in session/cookie for validation in callback
	// For now, we'll use a simple approach - in production, use proper session storage
	c.SetCookie("oauth_state", state, 600, "/", "", false, true) // 10 minutes, httpOnly

	url := config.GoogleOAuthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("access_type", "offline"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	// Get frontend URL for redirects
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	log.Printf("OAuth callback received - Query params: %v", c.Request.URL.Query())

	// Check for error parameter from Google (user denied permission)
	if errorParam := c.Query("error"); errorParam != "" {
		errorDescription := c.Query("error_description")
		if errorDescription == "" {
			errorDescription = "User denied permission"
		}
		log.Printf("OAuth error: %s - %s", errorParam, errorDescription)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=%s&error_description=%s", frontendURL, errorParam, errorDescription))
		return
	}

	// Validate state parameter to prevent CSRF attacks
	receivedState := c.Query("state")
	storedState, err := c.Cookie("oauth_state")
	if err != nil || receivedState == "" || receivedState != storedState {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=invalid_state&error_description=Invalid or missing state parameter", frontendURL))
		return
	}

	// Clear the state cookie after validation
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=missing_code&error_description=Missing authorization code", frontendURL))
		return
	}

	token, err := config.GoogleOAuthConfig.Exchange(c, code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=code_exchange_failed&error_description=Failed to exchange authorization code", frontendURL))
		return
	}

	client := config.GoogleOAuthConfig.Client(c, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil || resp.StatusCode != http.StatusOK {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=profile_fetch_failed&error_description=Failed to fetch user profile", frontendURL))
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
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=invalid_profile&error_description=Invalid profile response", frontendURL))
		return
	}

	if gu.Email == "" || !gu.EmailVerified {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=email_not_verified&error_description=Email not verified with Google", frontendURL))
		return
	}

	email := strings.ToLower(gu.Email)
	var user models.User
	if err := h.svc.DB().Where("email = ?", email).First(&user).Error; err == nil {
		// Update existing user
		user.Provider = "google"
		user.ProviderID = gu.Sub
		user.FirstName = gu.GivenName
		user.LastName = gu.FamilyName
		user.AvatarURL = gu.Picture
		user.EmailVerified = true
		now := time.Now()
		user.LastLogin = &now
		if err := h.svc.DB().Save(&user).Error; err != nil {
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=database_error&error_description=Failed to update user", frontendURL))
			return
		}
	} else {
		// Create new user
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
			c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=database_error&error_description=Failed to create user", frontendURL))
			return
		}
	}

	log.Printf("OAuth: Generating JWT token for user: %s", user.Email)
	jwt, err := util.GenerateToken(user.ID, user.Email, user.Provider)
	if err != nil {
		log.Printf("OAuth: JWT generation failed: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=token_generation_failed&error_description=Failed to generate JWT token", frontendURL))
		return
	}

	// Redirect directly to home page with JWT token
	redirectURL := fmt.Sprintf("%s/?token=%s", frontendURL, jwt)
	log.Printf("OAuth success - Redirecting to: %s", redirectURL)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

// OAuthStatus provides debugging information about OAuth state
func (h *AuthHandler) OAuthStatus(c *gin.Context) {
	state, err := c.Cookie("oauth_state")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"oauth_state": "not_set",
			"message":     "No OAuth state cookie found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"oauth_state": state,
		"message":     "OAuth state cookie found",
	})
}

// TestJWT provides debugging information about JWT token generation
func (h *AuthHandler) TestJWT(c *gin.Context) {
	// Test JWT generation with a proper UUID
	testUserID := uuid.New()
	testToken, err := util.GenerateToken(testUserID, "test@example.com", "test")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate test token",
			"details": err.Error(),
		})
		return
	}

	// Test JWT validation
	claims, err := util.ValidateToken(testToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to validate test token",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "JWT test successful",
		"token":        testToken,
		"claims":       claims,
		"test_user_id": testUserID.String(),
	})
}

// Me returns the authenticated user's profile based on JWT claims in context
func (h *AuthHandler) Me(c *gin.Context) {
	val, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Unauthorized"}})
		return
	}

	claims, ok := val.(*util.JWTClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "Invalid claims in context"}})
		return
	}

	var user models.User
	if err := h.svc.DB().WithContext(c.Request.Context()).Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: "User not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_500", Message: "Failed to fetch user", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, models.UserProfileResponse{
		Success: true,
		Message: "Authenticated",
		Data:    &user,
	})
}
