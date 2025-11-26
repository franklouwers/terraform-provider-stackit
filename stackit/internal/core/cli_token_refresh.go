package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// STACKIT OAuth2 token endpoint
	tokenEndpoint = "https://accounts.stackit.cloud/oauth2/token"
	// CLI client ID for OAuth2
	cliClientID = "stackit-cli-0000-0000-000000000001"
)

// RefreshTokenResponse represents the response from the token refresh endpoint
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// RefreshAccessToken refreshes an expired access token using the refresh token.
// It updates the credentials in place and writes them back to storage.
func RefreshAccessToken(creds *ProviderCredentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}

	if creds.RefreshToken == "" {
		return fmt.Errorf("refresh token is empty")
	}

	// Build refresh request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", creds.RefreshToken)
	data.Set("client_id", cliClientID)

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result RefreshTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Update credentials
	creds.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		creds.RefreshToken = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		creds.SessionExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	}

	// Write back to storage
	if err := WriteCLICredentials(creds); err != nil {
		return fmt.Errorf("write refreshed credentials: %w", err)
	}

	return nil
}

// IsTokenExpired checks if the access token has expired
func IsTokenExpired(creds *ProviderCredentials) bool {
	if creds == nil {
		return true
	}

	if creds.SessionExpiresAt.IsZero() {
		// No expiry time, assume valid
		return false
	}

	// Consider expired if within 5 minutes of expiry (safety margin)
	return time.Now().Add(5 * time.Minute).After(creds.SessionExpiresAt)
}

// EnsureValidToken checks if the token is expired and refreshes it if needed
func EnsureValidToken(creds *ProviderCredentials) error {
	if !IsTokenExpired(creds) {
		return nil
	}

	return RefreshAccessToken(creds)
}
