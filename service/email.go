package service

import (
	"fmt"
	"html"
	"log"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
)

type EmailService struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	fromEmail    string
	toEmail      string
}

var (
	emailServiceInstance *EmailService
	emailServiceOnce     sync.Once
)

// GetEmailService returns a singleton instance of EmailService
// This ensures we only read environment variables once and reuse the service
func GetEmailService() *EmailService {
	emailServiceOnce.Do(func() {
		emailServiceInstance = &EmailService{
			smtpHost:     getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
			smtpPort:     getEnvOrDefault("SMTP_PORT", "587"),
			smtpUsername: os.Getenv("SMTP_USERNAME"),
			smtpPassword: os.Getenv("SMTP_PASSWORD"),
			fromEmail:    os.Getenv("SMTP_FROM_EMAIL"),
			toEmail:      os.Getenv("SUPPORT_EMAIL"),
		}
	})
	return emailServiceInstance
}

// NewEmailService creates a new EmailService instance (for testing or custom config)
// For production use, prefer GetEmailService() singleton
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

var emailValidator = validator.New()

// validateEmailAddress validates email using proper email validation
func validateEmailAddress(email string) error {
	return emailValidator.Var(email, "required,email")
}

// sanitizeEmailAddress validates and sanitizes email address for use in headers
func sanitizeEmailAddress(email string) (string, error) {
	// Validate email format using proper validator
	if err := validateEmailAddress(email); err != nil {
		return "", fmt.Errorf("invalid email format: %w", err)
	}

	// Sanitize for header injection
	sanitized := sanitizeEmailHeader(email)
	if sanitized == "" {
		return "", fmt.Errorf("email cannot be empty")
	}

	return sanitized, nil
}

// escapeHTML escapes HTML special characters for safe display
func escapeHTML(text string) string {
	return html.EscapeString(text)
}

// generateHTMLTemplate generates HTML email template
func generateHTMLTemplate(name, email, message string) string {
	escapedName := escapeHTML(name)
	escapedEmail := escapeHTML(email)
	escapedMessage := escapeHTML(message)
	escapedMessageFormatted := strings.ReplaceAll(escapedMessage, "\n", "<br>")
	timestamp := time.Now().Format("January 2, 2006 at 3:04 PM")

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Support Request</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<div style="background-color: #f4f4f4; padding: 20px; border-radius: 5px; margin-bottom: 20px;">
		<h2 style="color: #2c3e50; margin-top: 0;">New Support Request Received</h2>
		<p style="color: #7f8c8d; font-size: 14px; margin: 0;">Received on %s</p>
	</div>
	
	<div style="background-color: #ffffff; padding: 20px; border: 1px solid #ddd; border-radius: 5px; margin-bottom: 20px;">
		<div style="margin-bottom: 15px;">
			<strong style="color: #2c3e50; display: inline-block; width: 80px;">From:</strong>
			<span style="color: #34495e;">%s</span>
		</div>
		<div style="margin-bottom: 15px;">
			<strong style="color: #2c3e50; display: inline-block; width: 80px;">Email:</strong>
			<a href="mailto:%s" style="color: #3498db; text-decoration: none;">%s</a>
		</div>
	</div>
	
	<div style="background-color: #ffffff; padding: 20px; border: 1px solid #ddd; border-radius: 5px; margin-bottom: 20px;">
		<h3 style="color: #2c3e50; margin-top: 0; border-bottom: 2px solid #3498db; padding-bottom: 10px;">Message</h3>
		<div style="color: #34495e; white-space: pre-wrap;">%s</div>
	</div>
	
	<div style="background-color: #ecf0f1; padding: 15px; border-radius: 5px; font-size: 12px; color: #7f8c8d; text-align: center;">
		<p style="margin: 0;">This email was sent from the support form on your website.</p>
		<p style="margin: 5px 0 0 0;">Reply to this email to respond directly to <strong>%s</strong>.</p>
	</div>
</body>
</html>`, timestamp, escapedName, escapedEmail, escapedEmail, escapedMessageFormatted, escapedEmail)
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
	sanitizedEmail, err := sanitizeEmailAddress(email)
	if err != nil {
		return fmt.Errorf("email validation failed: %w", err)
	}
	sanitizedMessage := strings.ReplaceAll(message, "\x00", "") // Remove null bytes from message body

	// Email subject - sanitized to prevent header injection
	subject := fmt.Sprintf("Support Request from %s", sanitizedName)

	// Generate plain text and HTML versions
	plainTextBody := es.generatePlainTextBody(sanitizedName, sanitizedEmail, sanitizedMessage)
	htmlBody := generateHTMLTemplate(sanitizedName, sanitizedEmail, sanitizedMessage)

	// Create multipart email (both plain text and HTML)
	emailBody := es.createMultipartEmail(plainTextBody, htmlBody)

	// Build email headers
	headers := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Reply-To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n",
		es.fromEmail, es.toEmail, sanitizedEmail, subject)

	// Combine headers and body
	fullEmail := headers + emailBody

	// SMTP authentication
	auth := smtp.PlainAuth("", es.smtpUsername, es.smtpPassword, es.smtpHost)

	// Send email
	addr := fmt.Sprintf("%s:%s", es.smtpHost, es.smtpPort)
	err = smtp.SendMail(addr, auth, es.fromEmail, []string{es.toEmail}, []byte(fullEmail))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// generatePlainTextBody generates plain text version of the email
func (es *EmailService) generatePlainTextBody(name, email, message string) string {
	return fmt.Sprintf("New Support Request Received\n\n"+
		"From: %s <%s>\n\n"+
		"Message:\n%s\n\n"+
		"---\n"+
		"This email was sent from the support form on your website.\n"+
		"Reply to this email to respond directly to %s.\n",
		name, email, message, email)
}

// createMultipartEmail creates a multipart email with both plain text and HTML versions
func (es *EmailService) createMultipartEmail(plainText, html string) string {
	boundary := "----=_NextPart_" + fmt.Sprintf("%d", time.Now().UnixNano())

	body := fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n"+
		"\r\n"+
		"--%s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"Content-Transfer-Encoding: 8bit\r\n"+
		"\r\n"+
		"%s\r\n"+
		"\r\n"+
		"--%s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"Content-Transfer-Encoding: 8bit\r\n"+
		"\r\n"+
		"%s\r\n"+
		"\r\n"+
		"--%s--\r\n",
		boundary, boundary, plainText, boundary, html, boundary)

	return body
}

// SendSupportEmailAsync sends email asynchronously in a goroutine
// This allows the HTTP handler to return immediately without waiting for SMTP
// Errors are logged but not returned to the caller
func (es *EmailService) SendSupportEmailAsync(name, email, message string) {
	go func() {
		if err := es.SendSupportEmail(name, email, message); err != nil {
			log.Printf("event=async_email_error name=%s email=%s err=%v", name, email, err)
		} else {
			log.Printf("event=async_email_sent name=%s email=%s", name, email)
		}
	}()
}
