package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// ProviderCredentials represents OAuth credentials stored by the STACKIT CLI
// for provider authentication (e.g., after running 'stackit auth provider login')
type ProviderCredentials struct {
	AccessToken          string
	RefreshToken         string
	Email                string
	SessionExpiresAt     time.Time
	AuthFlowType         string
	SourceProfile        string // Which profile these creds came from
	StorageLocationUsed  string // "keyring" or "file"
}

const (
	// Keychain service name prefix used by STACKIT CLI for provider auth
	keychainServicePrefix = "stackit-cli-provider"

	// Keychain account names
	keychainAccessToken       = "access_token"
	keychainRefreshToken      = "refresh_token"
	keychainUserEmail         = "user_email"
	keychainSessionExpiry     = "session_expires_at_unix"
	keychainAuthFlowType      = "auth_flow_type"

	// Default profile name
	defaultProfile = "default"
)

// getActiveProfile determines which CLI profile to use
// Priority: 1) explicit override, 2) STACKIT_CLI_PROFILE env var, 3) ~/.config/stackit/cli-profile.txt, 4) "default"
func getActiveProfile(profileOverride string) (string, error) {
	// 1. Explicit override from provider config
	if profileOverride != "" {
		return profileOverride, nil
	}

	// 2. Environment variable
	if profile := os.Getenv("STACKIT_CLI_PROFILE"); profile != "" {
		return profile, nil
	}

	// 3. Profile config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	profilePath := filepath.Join(homeDir, ".config", "stackit", "cli-profile.txt")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		// File doesn't exist, use default profile
		if os.IsNotExist(err) {
			return defaultProfile, nil
		}
		return "", fmt.Errorf("read profile file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

// getKeyringServiceName returns the keyring service name for a profile
func getKeyringServiceName(profile string) string {
	if profile == defaultProfile {
		return keychainServicePrefix
	}
	return fmt.Sprintf("%s/%s", keychainServicePrefix, profile)
}

// getFilePath returns the storage file path for a profile
func getFilePath(profile string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	if profile == defaultProfile {
		return filepath.Join(homeDir, ".stackit", "cli-provider-auth-storage.txt"), nil
	}
	return filepath.Join(homeDir, ".stackit", "profiles", profile, "cli-provider-auth-storage.txt"), nil
}

// ReadCLICredentials reads provider credentials from the STACKIT CLI storage.
// It first attempts to read from the system keychain, and falls back to reading from a Base64-encoded
// JSON file if the keychain is not available or fails.
//
// profileOverride allows specifying a profile explicitly (e.g., from Terraform config).
// If empty, uses STACKIT_CLI_PROFILE env var, then ~/.config/stackit/cli-profile.txt, then "default".
func ReadCLICredentials(profileOverride string) (*ProviderCredentials, error) {
	// Determine active profile
	profile, err := getActiveProfile(profileOverride)
	if err != nil {
		return nil, fmt.Errorf("determine active profile: %w", err)
	}

	// Try keyring first (primary storage method)
	creds, err := readFromKeyring(profile)
	if err == nil {
		creds.SourceProfile = profile
		creds.StorageLocationUsed = "keyring"
		return creds, nil
	}

	// Fall back to Base64-encoded JSON file
	creds, fileErr := readFromFile(profile)
	if fileErr == nil {
		creds.SourceProfile = profile
		creds.StorageLocationUsed = "file"
		return creds, nil
	}

	// Both methods failed - return a combined error message
	return nil, fmt.Errorf("failed to read CLI credentials from keyring (%v) or file (%v). Please run 'stackit auth provider login' first", err, fileErr)
}

// readFromKeychain reads provider credentials from the system keychain
// CLI stores each field as a separate keyring entry
func readFromKeychain(profile string) (*ProviderCredentials, error) {
	serviceName := getKeyringServiceName(profile)

	// Read access token (required)
	accessToken, err := keyring.Get(serviceName, keychainAccessToken)
	if err != nil {
		return nil, fmt.Errorf("get access_token: %w", err)
	}

	// Read refresh token (required)
	refreshToken, err := keyring.Get(serviceName, keychainRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("get refresh_token: %w", err)
	}

	// Read user email (required)
	email, err := keyring.Get(serviceName, keychainUserEmail)
	if err != nil {
		return nil, fmt.Errorf("get user_email: %w", err)
	}

	creds := &ProviderCredentials{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Email:        email,
	}

	// Read expiry (optional)
	if expiryStr, err := keyring.Get(serviceName, keychainSessionExpiry); err == nil {
		if expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
			creds.SessionExpiresAt = time.Unix(expiryUnix, 0)
		}
	}

	// Read auth flow type (optional)
	if authFlow, err := keyring.Get(serviceName, keychainAuthFlowType); err == nil {
		creds.AuthFlowType = authFlow
	}

	return creds, nil
}

// readFromFile reads provider credentials from the Base64-encoded JSON file fallback
func readFromFile(profile string) (*ProviderCredentials, error) {
	filePath, err := getFilePath(profile)
	if err != nil {
		return nil, fmt.Errorf("get file path: %w", err)
	}

	// Read Base64-encoded content
	contentEncoded, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found at %s", filePath)
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Decode from Base64
	contentBytes, err := base64.StdEncoding.DecodeString(string(contentEncoded))
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	// Parse JSON
	var data map[string]string
	if err := json.Unmarshal(contentBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	// Extract required fields
	accessToken, ok := data["access_token"]
	if !ok || accessToken == "" {
		return nil, fmt.Errorf("access_token not found in file")
	}

	refreshToken, ok := data["refresh_token"]
	if !ok || refreshToken == "" {
		return nil, fmt.Errorf("refresh_token not found in file")
	}

	email, ok := data["user_email"]
	if !ok || email == "" {
		return nil, fmt.Errorf("user_email not found in file")
	}

	creds := &ProviderCredentials{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Email:        email,
	}

	// Parse expiry (optional)
	if expiryStr, ok := data["session_expires_at_unix"]; ok {
		if expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
			creds.SessionExpiresAt = time.Unix(expiryUnix, 0)
		}
	}

	// Auth flow type (optional)
	if authFlow, ok := data["auth_flow_type"]; ok {
		creds.AuthFlowType = authFlow
	}

	return creds, nil
}

// IsAuthenticated checks if valid CLI provider credentials exist.
func IsAuthenticated(profileOverride string) bool {
	creds, err := ReadCLICredentials(profileOverride)
	if err != nil {
		return false
	}

	// Check if credentials exist and have an access token
	return creds != nil && creds.AccessToken != ""
}

// WriteCLICredentials writes provider credentials back to storage (for token refresh).
// It writes to the same location where credentials were read from (keyring or file).
func WriteCLICredentials(creds *ProviderCredentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}

	profile := creds.SourceProfile
	if profile == "" {
		profile = defaultProfile
	}

	// Try to write to keyring first
	if err := writeToKeyring(profile, creds); err == nil {
		return nil
	}

	// Fall back to file
	return writeToFile(profile, creds)
}

// writeToKeyring writes credentials to the system keyring
func writeToKeyring(profile string, creds *ProviderCredentials) error {
	serviceName := getKeyringServiceName(profile)

	// Write required fields
	if err := keyring.Set(serviceName, keychainAccessToken, creds.AccessToken); err != nil {
		return fmt.Errorf("set access_token: %w", err)
	}

	if err := keyring.Set(serviceName, keychainRefreshToken, creds.RefreshToken); err != nil {
		return fmt.Errorf("set refresh_token: %w", err)
	}

	if err := keyring.Set(serviceName, keychainUserEmail, creds.Email); err != nil {
		return fmt.Errorf("set user_email: %w", err)
	}

	// Write optional fields
	if !creds.SessionExpiresAt.IsZero() {
		expiryStr := fmt.Sprintf("%d", creds.SessionExpiresAt.Unix())
		keyring.Set(serviceName, keychainSessionExpiry, expiryStr)
	}

	if creds.AuthFlowType != "" {
		keyring.Set(serviceName, keychainAuthFlowType, creds.AuthFlowType)
	}

	return nil
}

// writeToFile writes credentials to the Base64-encoded JSON file
func writeToFile(profile string, creds *ProviderCredentials) error {
	filePath, err := getFilePath(profile)
	if err != nil {
		return fmt.Errorf("get file path: %w", err)
	}

	// Read existing file to preserve other fields
	var data map[string]string
	if existingContent, err := os.ReadFile(filePath); err == nil {
		if contentBytes, err := base64.StdEncoding.DecodeString(string(existingContent)); err == nil {
			json.Unmarshal(contentBytes, &data)
		}
	}

	if data == nil {
		data = make(map[string]string)
	}

	// Update credentials
	data["access_token"] = creds.AccessToken
	data["refresh_token"] = creds.RefreshToken
	data["user_email"] = creds.Email

	if !creds.SessionExpiresAt.IsZero() {
		data["session_expires_at_unix"] = fmt.Sprintf("%d", creds.SessionExpiresAt.Unix())
	}

	if creds.AuthFlowType != "" {
		data["auth_flow_type"] = creds.AuthFlowType
	}

	// Encode and write
	newContent, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(newContent)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	return os.WriteFile(filePath, []byte(encoded), 0600)
}
