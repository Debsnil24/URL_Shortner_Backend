package service

import (
	"encoding/base64"
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

// getLogoBase64 returns the base64-encoded SVG logo for email compatibility
func getLogoBase64() string {
	// SVG logo as base64-encoded data URI for better email client compatibility
	svgLogo := `<svg width="164" height="43" viewBox="0 0 164 43" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M1.305 43C0.771667 43 0.505 42.7333 0.505 42.2V36.184C0.505 35.9067 0.515667 35.6827 0.537 35.512C0.579667 35.3413 0.707667 35.2027 0.921 35.096L9.305 31.256V25.56L1.049 20.44C0.686333 20.2267 0.505 19.9067 0.505 19.48V7.736C0.505 7.39467 0.526333 7.14933 0.569 7C0.611667 6.82933 0.771667 6.648 1.049 6.456L9.465 0.759998C9.67833 0.631997 9.87033 0.535997 10.041 0.471996C10.233 0.386664 10.4677 0.343998 10.745 0.343998H19.385C19.9183 0.343998 20.185 0.610664 20.185 1.144V6.584C20.185 7.05333 20.025 7.37333 19.705 7.544L11.321 11V16.92L19.705 21.72C20.025 21.8693 20.185 22.2107 20.185 22.744V36.664C20.185 36.8347 20.153 37.016 20.089 37.208C20.0463 37.4 19.9397 37.5387 19.769 37.624L9.721 42.616C9.593 42.68 9.433 42.7653 9.241 42.872C9.07033 42.9573 8.89967 43 8.729 43H1.305ZM27.7025 43C27.1692 43 26.9025 42.7333 26.9025 42.2V1.144C26.9025 0.610664 27.1692 0.343998 27.7025 0.343998H45.8785C46.3905 0.343998 46.8172 0.514665 47.1585 0.855999L53.5585 7.256C53.9212 7.61867 54.1025 8.04533 54.1025 8.536V42.2C54.1025 42.7333 53.8358 43 53.3025 43H44.2465C43.7132 43 43.4465 42.7333 43.4465 42.2V8.856H37.5585V42.2C37.5585 42.7333 37.2918 43 36.7585 43H27.7025ZM62.5775 43C62.0442 43 61.7775 42.7333 61.7775 42.2V1.144C61.7775 0.610664 62.0442 0.343998 62.5775 0.343998H71.6335C72.1668 0.343998 72.4335 0.610664 72.4335 1.144V42.2C72.4335 42.7333 72.1668 43 71.6335 43H62.5775ZM80.89 43C80.3567 43 80.09 42.7333 80.09 42.2V1.144C80.09 0.610664 80.3567 0.343998 80.89 0.343998H98.586C99.098 0.343998 99.5247 0.514665 99.866 0.855999L106.266 7.256C106.629 7.61867 106.81 8.04533 106.81 8.536V19.576C106.81 20.088 106.629 20.5147 106.266 20.856L99.866 27.256C99.5247 27.6187 99.098 27.8 98.586 27.8H90.746V42.2C90.746 42.7333 90.4793 43 89.946 43H80.89ZM93.146 19.096C94.9167 19.096 96.122 18.744 96.762 18.04C97.4233 17.336 97.754 16.0027 97.754 14.04C97.754 12.0773 97.4233 10.744 96.762 10.04C96.122 9.336 94.9167 8.984 93.146 8.984C91.3967 8.984 90.1913 9.336 89.53 10.04C88.89 10.744 88.57 12.0773 88.57 14.04C88.57 16.0027 88.89 17.336 89.53 18.04C90.1913 18.744 91.3967 19.096 93.146 19.096ZM112.765 43C112.232 43 111.965 42.7333 111.965 42.2V1.144C111.965 0.610664 112.232 0.343998 112.765 0.343998H121.821C122.354 0.343998 122.621 0.610664 122.621 1.144V34.456H130.685C131.218 34.456 131.485 34.7227 131.485 35.256V42.2C131.485 42.7333 131.218 43 130.685 43H112.765ZM145.006 43C144.473 43 144.206 42.7333 144.206 42.2V29.72L135.63 21.176C135.481 21.0053 135.385 20.856 135.342 20.728C135.321 20.5787 135.31 20.408 135.31 20.216V1.144C135.31 0.610664 135.577 0.343998 136.11 0.343998H145.166C145.699 0.343998 145.966 0.610664 145.966 1.144V20.504H153.134V1.144C153.134 0.610664 153.401 0.343998 153.934 0.343998H162.99C163.523 0.343998 163.79 0.610664 163.79 1.144V20.216C163.79 20.408 163.779 20.5787 163.758 20.728C163.737 20.856 163.641 21.0053 163.47 21.176L154.926 29.72V42.2C154.926 42.7333 154.659 43 154.126 43H145.006Z" fill="black"/></svg>`
	return base64.StdEncoding.EncodeToString([]byte(svgLogo))
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
