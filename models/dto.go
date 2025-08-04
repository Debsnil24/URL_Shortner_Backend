package models

type ShortenURLRequest struct {
    URL string `json:"url" binding:"required,url"`
}

type ShortenURLResponse struct {
	ShortenedURL string `json:"shortened_url"`
	OriginalURL  string `json:"original_url"`
	ShortCode    string `json:"short_code"`
}