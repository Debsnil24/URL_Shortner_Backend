package handler

import (
	"net/http"
	"strings"

	"github.com/Debsnil24/URL_Shortner.git/models"
	"github.com/Debsnil24/URL_Shortner.git/controller"
	"github.com/gin-gonic/gin"
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
