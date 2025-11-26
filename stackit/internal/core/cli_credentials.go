package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// ProviderCredentials represents OAuth credentials stored by the STACKIT CLI
// for provider authentication (e.g., after running 'stackit auth provider login')
type ProviderCredentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TokenType    string    `json:"token_type,omitempty"`
}

const (
	// Keychain service name used by STACKIT CLI
	// Must match the service name used by the CLI
	keychainService = "stackit-cli"
	// Keychain key for provider credentials
	// Must match the key used by the CLI
	keychainProviderKey = "provider-credentials"
)

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

// ReadCLICredentials reads provider credentials from the STACKIT CLI storage.
// It first attempts to read from the system keychain (Windows Credential Manager,
// macOS Keychain, Linux Secret Service), and falls back to reading from a JSON
// file if the keychain is not available or fails.
//
// This matches the CLI's storage strategy for maximum compatibility.
func ReadCLICredentials() (*ProviderCredentials, error) {
	// Try keychain first (primary storage method)
	creds, err := readFromKeychain()
	if err == nil {
		return creds, nil
	}

	// Fall back to JSON file
	creds, fileErr := readFromFile()
	if fileErr == nil {
		return creds, nil
	}

	// Both methods failed - return a combined error message
	return nil, fmt.Errorf("failed to read CLI credentials from keychain (%v) or file (%v). Please run 'stackit auth provider login' first", err, fileErr)
}

// readFromKeychain reads provider credentials from the system keychain
func readFromKeychain() (*ProviderCredentials, error) {
	// Get JSON string from keychain
	data, err := keyring.Get(keychainService, keychainProviderKey)
	if err != nil {
		return nil, fmt.Errorf("keychain read failed: %w", err)
	}

	// Parse JSON credentials
	var creds ProviderCredentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return nil, fmt.Errorf("failed to parse keychain credentials: %w", err)
	}

	return &creds, nil
}

// readFromFile reads provider credentials from the JSON file fallback
func readFromFile() (*ProviderCredentials, error) {
	credPath, err := GetCLICredentialsPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine credentials path: %w", err)
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found at %s", credPath)
		}
		return nil, fmt.Errorf("failed to read credentials from %s: %w", credPath, err)
	}

	var creds ProviderCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &creds, nil
}

// IsAuthenticated checks if valid CLI provider credentials exist.
// It checks both keychain and file storage.
func IsAuthenticated() bool {
	creds, err := ReadCLICredentials()
	if err != nil {
		return false
	}

	// Check if credentials exist and have an access token
	return creds != nil && creds.AccessToken != ""
}

// GetStorageLocation returns information about where credentials are stored.
// This is useful for debugging and user feedback.
func GetStorageLocation() string {
	// Check keychain first
	_, err := keyring.Get(keychainService, keychainProviderKey)
	if err == nil {
		return "system keychain"
	}

	// Check file
	credPath, err := GetCLICredentialsPath()
	if err == nil {
		if _, err := os.Stat(credPath); err == nil {
			return credPath
		}
	}

	return "not found"
}
