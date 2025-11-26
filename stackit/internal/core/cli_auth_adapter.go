package core

import (
	"net/http"

	cliAuth "github.com/stackitcloud/stackit-cli/pkg/auth"
)

// CLIAuthAdapter implements the SDK's CLIAuthProvider interface to bridge
// between the STACKIT CLI and STACKIT SDK without creating circular dependencies.
//
// This adapter allows the Terraform Provider to integrate CLI authentication
// by implementing the interface expected by the SDK, while the SDK itself
// doesn't need to import the CLI package.
type CLIAuthAdapter struct{}

// NewCLIAuthAdapter creates a new CLI auth adapter.
func NewCLIAuthAdapter() *CLIAuthAdapter {
	return &CLIAuthAdapter{}
}

// IsAuthenticated checks if CLI provider credentials exist.
// This calls the CLI's IsProviderAuthenticated function to determine
// if the user has run 'stackit auth provider login'.
func (a *CLIAuthAdapter) IsAuthenticated() bool {
	return cliAuth.IsProviderAuthenticated()
}

// GetAuthFlow returns an http.RoundTripper configured with CLI authentication.
// The returned RoundTripper:
// - Automatically checks token expiration on every HTTP request
// - Refreshes tokens when needed
// - Writes refreshed tokens back to CLI storage
// - Is thread-safe for concurrent use
func (a *CLIAuthAdapter) GetAuthFlow() (http.RoundTripper, error) {
	// Pass nil printer since Terraform provider doesn't need CLI output
	return cliAuth.ProviderAuthFlow(nil)
}
