package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

// loginTemplate is a cached template to avoid parsing on every request
var (
	loginTemplate *template.Template
	templateOnce  sync.Once
	templateErr   error
)

// initLoginTemplate initializes the login template (thread-safe)
func initLoginTemplate() error {
	templateOnce.Do(func() {
		// Get the template path
		templatePath := filepath.Join("templates", "login.html")
		
		// Check if template file exists
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			templateErr = err
			return
		}
		
		// Parse the template
		tmpl, err := template.ParseFiles(templatePath)
		if err != nil {
			templateErr = err
			return
		}
		
		loginTemplate = tmpl
	})
	return templateErr
}

// LoginPage serves a simple HTML login page for Swagger authentication
func LoginPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		redirectParam := c.DefaultQuery("redirect", "/swagger")
		// Decode the redirect URL if it was encoded
		redirectURL, err := url.QueryUnescape(redirectParam)
		if err != nil {
			// URL decode error - use default redirect
			redirectURL = "/swagger"
		} else if redirectURL == "" {
			// Empty redirect URL after decoding - use default
			redirectURL = "/swagger"
		}
		// Ensure redirect URL is properly formatted
		if redirectURL == "" || redirectURL == "/" {
			redirectURL = "/swagger"
		}

		// Initialize template if not already loaded
		if loginTemplate == nil {
			if err := initLoginTemplate(); err != nil {
				c.String(http.StatusInternalServerError, "Template not available: %v", err)
				return
			}
		}

		// Prepare template data with properly escaped redirect URL
		data := struct {
			RedirectURL string
		}{
			RedirectURL: template.JSEscapeString(redirectURL),
		}

		// Execute template into buffer
		var buf bytes.Buffer
		if err := loginTemplate.Execute(&buf, data); err != nil {
			c.String(http.StatusInternalServerError, "Template execution error: %v", err)
			return
		}

		// Write the rendered HTML
		c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
	}
}

