package service

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

type EmailService struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	fromEmail    string
	toEmail      string
}

func NewEmailService() *EmailService {
	return &EmailService{
		smtpHost:     getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
		smtpPort:     getEnvOrDefault("SMTP_PORT", "587"),
		smtpUsername: os.Getenv("SMTP_USERNAME"),
		smtpPassword: os.Getenv("SMTP_PASSWORD"),
		fromEmail:    os.Getenv("SMTP_FROM_EMAIL"),
		toEmail:      os.Getenv("SUPPORT_EMAIL"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// sanitizeEmailHeader removes dangerous characters that could be used for header injection
// Prevents email header injection attacks by removing \r, \n, and null bytes
func sanitizeEmailHeader(input string) string {
	// Remove carriage returns, line feeds, and null bytes
	sanitized := strings.ReplaceAll(input, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")
	sanitized = strings.ReplaceAll(sanitized, "\x00", "")
	// Remove any remaining control characters
	sanitized = strings.Map(func(r rune) rune {
		if r < 32 && r != 9 { // Allow tab (9) but remove other control chars
			return -1
		}
		return r
	}, sanitized)
	return strings.TrimSpace(sanitized)
}

// sanitizeEmailAddress validates and sanitizes email address for use in headers
func sanitizeEmailAddress(email string) string {
	// Basic validation - remove dangerous characters
	sanitized := sanitizeEmailHeader(email)
	// Ensure it's a valid email format (basic check)
	if !strings.Contains(sanitized, "@") {
		return ""
	}
	return sanitized
}

func (es *EmailService) SendSupportEmail(name, email, message string) error {
	// Validate required configuration
	if es.smtpUsername == "" || es.smtpPassword == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}
	if es.fromEmail == "" {
		return fmt.Errorf("SMTP_FROM_EMAIL not configured")
	}
	if es.toEmail == "" {
		return fmt.Errorf("SUPPORT_EMAIL not configured")
	}

	// Sanitize inputs to prevent email header injection attacks
	sanitizedName := sanitizeEmailHeader(name)
	sanitizedEmail := sanitizeEmailAddress(email)
	sanitizedMessage := strings.ReplaceAll(message, "\x00", "") // Remove null bytes from message body

	// Validate sanitized email
	if sanitizedEmail == "" || !strings.Contains(sanitizedEmail, "@") {
		return fmt.Errorf("invalid email address")
	}

	// Email subject - sanitized to prevent header injection
	subject := fmt.Sprintf("Support Request from %s", sanitizedName)

	// Email body with proper headers
	// Note: From must be the authenticated email, but Reply-To is set to the submitter's email
	// All user inputs are sanitized to prevent header injection
	body := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Reply-To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"New Support Request Received\r\n"+
		"\r\n"+
		"From: %s <%s>\r\n"+
		"\r\n"+
		"Message:\r\n"+
		"%s\r\n"+
		"\r\n"+
		"---\r\n"+
		"This email was sent from the support form on your website.\r\n"+
		"Reply to this email to respond directly to %s.\r\n",
		es.fromEmail, es.toEmail, sanitizedEmail, subject, sanitizedName, sanitizedEmail, sanitizedMessage, sanitizedEmail)

	// SMTP authentication
	auth := smtp.PlainAuth("", es.smtpUsername, es.smtpPassword, es.smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%s", es.smtpHost, es.smtpPort)
	err := smtp.SendMail(addr, auth, es.fromEmail, []string{es.toEmail}, []byte(body))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
