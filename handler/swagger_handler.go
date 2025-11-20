package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// CustomSwaggerHandler wraps the standard Swagger handler and injects JavaScript
// to automatically fetch and set the Bearer token from the cookie-based auth
func CustomSwaggerHandler() gin.HandlerFunc {
	// Get the standard Swagger handler
	swaggerHandler := ginSwagger.WrapHandler(swaggerFiles.Handler)

	return func(c *gin.Context) {
		// Check if this is a request for the Swagger UI HTML page
		isHTMLRequest := c.Request.URL.Path == "/swagger/index.html" || 
		                 c.Request.URL.Path == "/swagger/" ||
		                 strings.HasSuffix(c.Request.URL.Path, ".html")
		
		if isHTMLRequest {
			// Create a custom response writer to intercept and modify the HTML
			w := &responseWriter{
				ResponseWriter: c.Writer,
				body:           []byte{},
			}
			c.Writer = w

			// Call the standard Swagger handler
			swaggerHandler(c)

			// If the response is HTML and we captured the body, inject our JavaScript
			contentType := w.Header().Get("Content-Type")
			if strings.Contains(contentType, "text/html") && len(w.body) > 0 {
				html := string(w.body)
				
				// Inject JavaScript before closing body tag
				jsInjection := `
<script>
(function() {
	// Store interval IDs to prevent duplicates and enable cleanup
	let swaggerCheckInterval = null;
	let topBarCheckInterval = null;
	let swaggerCheckTimeout = null;
	let topBarCheckTimeout = null;
	
	// Cleanup function to clear all intervals and timeouts
	function cleanupIntervals() {
		if (swaggerCheckInterval !== null) {
			clearInterval(swaggerCheckInterval);
			swaggerCheckInterval = null;
		}
		if (topBarCheckInterval !== null) {
			clearInterval(topBarCheckInterval);
			topBarCheckInterval = null;
		}
		if (swaggerCheckTimeout !== null) {
			clearTimeout(swaggerCheckTimeout);
			swaggerCheckTimeout = null;
		}
		if (topBarCheckTimeout !== null) {
			clearTimeout(topBarCheckTimeout);
			topBarCheckTimeout = null;
		}
	}
	
	// Function to get token from server and set it in Swagger UI
	function setSwaggerToken() {
		// Prevent multiple calls from creating duplicate intervals
		if (swaggerCheckInterval !== null) {
			return;
		}
		
		fetch('/api/swagger-token')
			.then(response => {
				if (!response.ok) {
					console.log('Swagger: Not authenticated or token unavailable');
					return;
				}
				return response.json();
			})
			.then(data => {
				if (data && data.token) {
					// Wait for Swagger UI to be ready
					swaggerCheckInterval = setInterval(() => {
						if (window.ui && window.ui.authActions) {
							clearInterval(swaggerCheckInterval);
							swaggerCheckInterval = null;
							
							// Set the Bearer token in Swagger UI
							window.ui.authActions.authorize({
								BearerAuth: {
									name: 'BearerAuth',
									schema: {
										type: 'apiKey',
										in: 'header',
										name: 'Authorization',
										description: 'Type "Bearer" followed by a space and JWT token.'
									},
									value: 'Bearer ' + data.token
								}
							});
							
							console.log('Swagger: Bearer token automatically set');
							
							// Add logout button to Swagger UI
							addLogoutButton();
						}
					}, 100);
					
					// Timeout after 5 seconds
					swaggerCheckTimeout = setTimeout(() => {
						if (swaggerCheckInterval !== null) {
							clearInterval(swaggerCheckInterval);
							swaggerCheckInterval = null;
						}
					}, 5000);
				}
			})
			.catch(error => {
				console.log('Swagger: Error fetching token:', error);
			});
	}
	
	// Function to add logout button to Swagger UI
	function addLogoutButton() {
		// Prevent multiple calls from creating duplicate intervals
		if (topBarCheckInterval !== null) {
			return;
		}
		
		// Check if logout button already exists
		if (document.getElementById('swagger-logout-btn')) {
			return;
		}
		
		// Wait for Swagger UI top bar to be ready
		topBarCheckInterval = setInterval(() => {
			const topbar = document.querySelector('.topbar');
			if (topbar) {
				clearInterval(topBarCheckInterval);
				topBarCheckInterval = null;
				
				// Double-check if logout button already exists (race condition guard)
				if (document.getElementById('swagger-logout-btn')) {
					return;
				}
				
				// Create logout button
				const logoutBtn = document.createElement('button');
				logoutBtn.id = 'swagger-logout-btn';
				logoutBtn.textContent = 'Logout';
				logoutBtn.style.cssText = 'margin-left: 10px; padding: 8px 16px; background: #f44336; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 14px;';
				logoutBtn.onmouseover = function() { this.style.background = '#d32f2f'; };
				logoutBtn.onmouseout = function() { this.style.background = '#f44336'; };
				logoutBtn.onclick = function(e) {
					e.preventDefault();
					logoutBtn.disabled = true;
					logoutBtn.textContent = 'Logging out...';
					
					fetch('/auth/swagger-logout', {
						method: 'POST',
						credentials: 'include'
					})
					.then(response => response.json())
					.then(data => {
						if (data.success) {
							// Clear Swagger UI authorization
							if (window.ui && window.ui.authActions) {
								window.ui.authActions.logout(['BearerAuth']);
							}
							// Redirect to login page
							window.location.href = '/auth/login-page?redirect=/swagger';
						} else {
							alert('Logout failed. Please try again.');
							logoutBtn.disabled = false;
							logoutBtn.textContent = 'Logout';
						}
					})
					.catch(error => {
						console.error('Logout error:', error);
						alert('Logout failed. Please try again.');
						logoutBtn.disabled = false;
						logoutBtn.textContent = 'Logout';
					});
				};
				
				// Insert logout button into topbar
				topbar.appendChild(logoutBtn);
			}
		}, 100);
		
		// Timeout after 5 seconds
		topBarCheckTimeout = setTimeout(() => {
			if (topBarCheckInterval !== null) {
				clearInterval(topBarCheckInterval);
				topBarCheckInterval = null;
			}
		}, 5000);
	}
	
	// Set token when page loads
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', setSwaggerToken);
	} else {
		setSwaggerToken();
	}
	
	// Cleanup on page unload to prevent memory leaks
	window.addEventListener('beforeunload', cleanupIntervals);
})();
</script>
`
				// Inject before closing body tag (optimized: single-pass search and replace)
				bodyTagIndex := strings.LastIndex(html, "</body>")
				if bodyTagIndex != -1 {
					// Found closing body tag, inject JavaScript before it
					html = html[:bodyTagIndex] + jsInjection + html[bodyTagIndex:]
				} else {
					// No body tag found, append at the end
					html += jsInjection
				}
				
				// Clear the original response and write modified HTML
				w.ResponseWriter.Header().Del("Content-Length")
				w.ResponseWriter.WriteHeader(w.statusCode)
				w.ResponseWriter.Write([]byte(html))
			}
		} else {
			// For non-HTML requests (CSS, JS, JSON), use standard handler
			swaggerHandler(c)
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture the response body
type responseWriter struct {
	gin.ResponseWriter
	body       []byte
	statusCode int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return len(b), nil // Don't write yet, we'll write modified content
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body = append(w.body, s...)
	return len(s), nil // Don't write yet, we'll write modified content
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	// Don't write header yet, we'll write it with modified content
}

