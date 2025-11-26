package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ProviderCredentials represents OAuth credentials stored by the STACKIT CLI
// for provider authentication (e.g., after running 'stackit auth provider login')
type ProviderCredentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type,omitempty"`
}

// GetCLICredentialsPath returns the path where STACKIT CLI stores provider credentials
func GetCLICredentialsPath() (string, error) {
	// Check STACKIT_CLI_CONFIG_DIR environment variable first
	if configDir := os.Getenv("STACKIT_CLI_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "provider-credentials.json"), nil
	}

	// Fall back to default location: ~/.stackit/provider-credentials.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".stackit", "provider-credentials.json"), nil
}

// ReadCLICredentials reads provider credentials from the STACKIT CLI storage
func ReadCLICredentials() (*ProviderCredentials, error) {
	credPath, err := GetCLICredentialsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine credentials path: %w", err)
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("CLI credentials not found at %s. Please run 'stackit auth provider login' first", credPath)
		}
		return nil, fmt.Errorf("failed to read credentials from %s: %w", credPath, err)
	}

	var creds ProviderCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// IsAuthenticated checks if valid CLI provider credentials exist
func IsAuthenticated() bool {
	creds, err := ReadCLICredentials()
	if err != nil {
		return false
	}

	// Check if credentials exist and have an access token
	return creds != nil && creds.AccessToken != ""
}
