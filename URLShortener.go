package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"koletackney.dev/urlshortener/Internal/storage"
	"koletackney.dev/urlshortener/ui/pages"
)

var encoder = base64.URLEncoding

func generateShortCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return encoder.EncodeToString(b)[:8], nil
}

func MakeEncodeURL(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		encodeURL(w, r, store)
	}
}

func encodeURL(w http.ResponseWriter, r *http.Request, store *storage.Store) {
	url := r.FormValue("URL")
	if url == "" {
		http.Error(w, "URL param is required", http.StatusBadRequest)
		return
	}

	// Check to see if the url is already shortened.
	info, err := store.URLInfo(r.Context(), url)
	if err == nil {
		// URL exists, check if it has an expiration date
		if !info.ExpiresAt.Valid {
			// No expiration date, return existing shortened URL
			renderURLCard(w, r, url, info.ShortCode)
			return
		}
		// Has expiration date, continue to create a new URL
	}

	// Generate a unique short code with retry logic
	const maxRetries = 10
	var shortCode string

	for i := 0; i < maxRetries; i++ {
		shortCode, err = generateShortCode()
		if err != nil {
			http.Error(w, "Failed to generate short code", http.StatusInternalServerError)
			return
		}

		// Try to store in database
		err = store.Insert(r.Context(), shortCode, url, nil)
		if err == nil {
			// Success!
			break
		}

		// If it's a duplicate, retry with a new code
		if errors.Is(err, storage.ErrDuplicateShortCode) {
			continue
		}

		// For any other error, fail immediately
		http.Error(w, "Failed to store URL", http.StatusInternalServerError)
		return
	}

	// If we exhausted all retries
	if err != nil {
		http.Error(w, "Failed to generate unique short code", http.StatusInternalServerError)
		return
	}

	renderURLCard(w, r, url, shortCode)
}

func renderURLCard(w http.ResponseWriter, r *http.Request, originalURL, shortCode string) {
	baseURL := publicBaseURL(r)
	newURL := fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), shortCode)
	urlCard := pages.URLCard(originalURL, newURL)
	err := urlCard.Render(r.Context(), w)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func MakeRedirectURL(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		redirectURL(w, r, store)
	}
}

func redirectURL(w http.ResponseWriter, r *http.Request, store *storage.Store) {
	// Get the short code from path parameter
	shortCode := r.PathValue("encodedIndex")
	if shortCode == "" {
		http.Error(w, "short code is required", http.StatusBadRequest)
		return
	}

	// Look up the URL in the database
	urlData, err := store.Get(r.Context(), shortCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Show missing URL page
			component := pages.MissingURL()
			err := component.Render(r.Context(), w)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect to the original URL
	http.Redirect(w, r, urlData.OriginalURL, http.StatusFound)
}

func publicBaseURL(r *http.Request) string {
	scheme := forwardedHeaderValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := forwardedHeaderValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

func forwardedHeaderValue(value string) string {
	if index := strings.Index(value, ","); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}
