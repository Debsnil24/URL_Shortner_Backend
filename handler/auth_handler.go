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

// setAuthCookie sets a secure HttpOnly cookie with the JWT token
func (h *AuthHandler) setAuthCookie(c *gin.Context, token string) {
	h.setCookieWithName(c, token, "auth_token")
}

// setSwaggerCookie sets a separate HttpOnly cookie for Swagger authentication
// This keeps Swagger login separate from frontend login
func (h *AuthHandler) setSwaggerCookie(c *gin.Context, token string) {
	h.setCookieWithName(c, token, "swagger_auth_token")
}

// setCookieWithName is a helper to set cookies with a specific name
func (h *AuthHandler) setCookieWithName(c *gin.Context, token string, cookieName string) {
	// Check if we're in production (HTTPS)
	isProduction := os.Getenv("ENV") == "production"

	// Get domain from environment or use current domain
	domain := os.Getenv("COOKIE_DOMAIN")
	if domain == "" && isProduction {
		domain = ".sniply.co.in" // Your main domain with leading dot
	}

	// For OAuth flows, we need to be more permissive with SameSite
	// Set SameSite=Lax to allow OAuth redirects
	sameSite := "Lax"

	// Build cookie string with all attributes
	cookieValue := fmt.Sprintf("%s=%s; Path=/; Max-Age=3600; HttpOnly; SameSite=%s",
		cookieName,
		token,
		sameSite)

	// Add domain if specified
	if domain != "" {
		cookieValue += fmt.Sprintf("; Domain=%s", domain)
	}

	// Add Secure flag in production (HTTPS only)
	if isProduction {
		cookieValue += "; Secure"
	}

	// Set cookie using manual header (allows SameSite attribute)
	c.Header("Set-Cookie", cookieValue)
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account with email and password. JWT token is set in HttpOnly cookie only (not returned in response body for security). For API clients, use cookie-based authentication or implement a separate token endpoint.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      models.RegisterRequest  true  "Registration details"
// @Success      201      {object}  models.AuthResponse
// @Failure      400      {object}  models.AuthResponse
// @Failure      409      {object}  models.AuthResponse
// @Failure      500      {object}  models.AuthResponse
// @Router       /auth/register [post]
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

	// Set HttpOnly cookie with JWT token
	if resp.Success && resp.Data != nil && resp.Data.Token != "" {
		h.setAuthCookie(c, resp.Data.Token)
		// Remove token from response for security - tokens are only in HttpOnly cookies
		resp.Data.Token = ""
	}

	c.JSON(http.StatusCreated, resp)
}

// Login godoc
// @Summary      Login user
// @Description  Authenticates a user with email and password. JWT token is set in HttpOnly cookie only (not returned in response body for security). For API clients, use cookie-based authentication or implement a separate token endpoint.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      models.LoginRequest  true  "Login credentials"
// @Success      200      {object}  models.AuthResponse
// @Failure      400      {object}  models.AuthResponse
// @Failure      401      {object}  models.AuthResponse
// @Failure      500      {object}  models.AuthResponse
// @Router       /auth/login [post]
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

	// Set HttpOnly cookie with JWT token
	if resp.Success && resp.Data != nil && resp.Data.Token != "" {
		// Check if this is a Swagger login (from login page or query param)
		isSwaggerLogin := c.Query("swagger") == "true" ||
			c.GetHeader("X-Swagger-Login") == "true" ||
			strings.Contains(c.GetHeader("Referer"), "/auth/login-page")

		if isSwaggerLogin {
			// Use separate cookie for Swagger to keep it isolated from frontend
			h.setSwaggerCookie(c, resp.Data.Token)
		} else {
			// Regular frontend login
			h.setAuthCookie(c, resp.Data.Token)
		}
		// Remove token from response for security - tokens are only in HttpOnly cookies
		resp.Data.Token = ""
	}

	c.JSON(http.StatusOK, resp)
}

// GoogleAuth godoc
// @Summary      Initiate Google OAuth
// @Description  Redirects to Google OAuth consent screen
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      307  {string}  string  "Redirect to Google"
// @Failure      500  {object}  models.AuthResponse
// @Router       /auth/google [get]
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

	// Check if we're in production
	isProduction := os.Getenv("ENV") == "production"
	domain := os.Getenv("COOKIE_DOMAIN")
	if domain == "" && isProduction {
		domain = ".sniply.co.in"
	}

	// Store state in session/cookie for validation in callback
	c.SetCookie("oauth_state", state, 600, "/", domain, isProduction, true) // 10 minutes, httpOnly

	url := config.GoogleOAuthConfig.AuthCodeURL(state,
		oauth2.SetAuthURLParam("access_type", "offline"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback godoc
// @Summary      Google OAuth callback
// @Description  Handles the OAuth callback from Google and creates/updates user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        code   query     string  true  "Authorization code from Google"
// @Param        state  query     string  true  "State parameter for CSRF protection"
// @Success      307    {string}  string  "Redirect to frontend"
// @Failure      307    {string}  string  "Redirect to frontend with error"
// @Router       /auth/google/callback [get]
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
	if err != nil {
		// Cookie not found or error reading cookie
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=invalid_state&error_description=State cookie not found", frontendURL))
		return
	}
	if receivedState == "" {
		// State parameter missing from callback
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=invalid_state&error_description=Missing state parameter", frontendURL))
		return
	}
	if receivedState != storedState {
		// State mismatch - possible CSRF attack
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=invalid_state&error_description=State mismatch", frontendURL))
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
	if err != nil {
		// Network error or request failed
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=profile_fetch_failed&error_description=Network error while fetching user profile: %v", frontendURL, err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// HTTP error status from Google API
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/?error=profile_fetch_failed&error_description=Failed to fetch user profile (HTTP %d)", frontendURL, resp.StatusCode))
		return
	}

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

	// Set HttpOnly cookie with JWT token
	h.setAuthCookie(c, jwt)
	log.Printf("OAuth: Set auth cookie for user %s", user.Email)

	// Redirect to home page (token is now in cookie, not URL)
	log.Printf("OAuth success - Redirecting to: %s", frontendURL)
	c.Redirect(http.StatusTemporaryRedirect, frontendURL)
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

// Me godoc
// @Summary      Get current user profile
// @Description  Returns the authenticated user's profile information
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.UserProfileResponse
// @Failure      401  {object}  models.AuthResponse
// @Failure      500  {object}  models.AuthResponse
// @Security     BearerAuth
// @Router       /auth/me [get]
// Me returns the authenticated user's profile based on JWT claims in context
func (h *AuthHandler) Me(c *gin.Context) {
	// Get userID from context (set by AuthRequired middleware)
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{Success: false, Error: &models.AuthError{Code: "AUTH_401", Message: err.Error()}})
		return
	}

	var user models.User
	if err := h.svc.DB().WithContext(c.Request.Context()).Where("id = ?", userID).First(&user).Error; err != nil {
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

// Logout godoc
// @Summary      Logout user
// @Description  Clears the authentication cookie and logs out the user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.AuthResponse
// @Router       /auth/logout [post]
// Logout clears the authentication cookie
func (h *AuthHandler) Logout(c *gin.Context) {
	// Check if this is a Swagger logout
	isSwaggerLogout := c.Query("swagger") == "true" ||
		c.GetHeader("X-Swagger-Logout") == "true" ||
		strings.Contains(c.GetHeader("Referer"), "/swagger")

	if isSwaggerLogout {
		// Clear Swagger auth cookie
		h.clearCookie(c, "swagger_auth_token")
		c.JSON(http.StatusOK, models.AuthResponse{
			Success: true,
			Message: "Logged out from Swagger successfully",
		})
		return
	}

	// Clear the frontend auth cookie
	h.clearCookie(c, "auth_token")
	c.JSON(http.StatusOK, models.AuthResponse{
		Success: true,
		Message: "Logged out successfully",
	})
}

// SwaggerLogout godoc
// @Summary      Logout from Swagger UI
// @Description  Clears the Swagger-specific authentication cookie (swagger_auth_token). This endpoint is used by Swagger UI's logout functionality. No authentication required.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  models.AuthResponse
// @Router       /auth/swagger-logout [post]
// SwaggerLogout clears only the Swagger authentication cookie
func (h *AuthHandler) SwaggerLogout(c *gin.Context) {
	h.clearCookie(c, "swagger_auth_token")
	c.JSON(http.StatusOK, models.AuthResponse{
		Success: true,
		Message: "Logged out from Swagger successfully",
	})
}

// clearCookie clears a cookie by setting it to expire immediately
func (h *AuthHandler) clearCookie(c *gin.Context, cookieName string) {
	isProduction := os.Getenv("ENV") == "production"
	domain := os.Getenv("COOKIE_DOMAIN")
	if domain == "" && isProduction {
		domain = ".sniply.co.in"
	}

	c.SetCookie(
		cookieName, // name
		"",         // value (empty)
		-1,         // maxAge (expire immediately)
		"/",        // path
		domain,     // domain
		false,      // secure (not needed for clearing)
		true,       // httpOnly
	)
}
