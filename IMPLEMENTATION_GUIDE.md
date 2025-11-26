# CLI Provider Authentication - Implementation Guide

This guide explains how to implement CLI provider authentication using an interface-based approach to avoid circular dependencies.

## Architecture Overview

```
┌─────────────────────────────────────────────┐
│ Terraform Provider (Integration Layer)      │
│ - Imports CLI package                       │
│ - Imports SDK package                       │
│ - Creates CLIAuthAdapter implementation     │
└─────────────────────────────────────────────┘
            │                    │
            │                    │
            ▼                    ▼
┌─────────────────┐    ┌─────────────────────┐
│ STACKIT CLI     │    │ STACKIT SDK         │
│ - Auth storage  │    │ - CLIAuthProvider   │
│ - OAuth flows   │    │   interface         │
│ - Token refresh │    │ - No CLI import     │
└─────────────────┘    └─────────────────────┘
```

**Key Concept:** The SDK defines a `CLIAuthProvider` interface but doesn't import the CLI. The Provider creates an adapter that implements this interface, bridging CLI and SDK without circular dependencies.

**Flow:**
1. User runs: `stackit auth provider login`
2. CLI stores credentials (keyring or file)
3. Provider creates `CLIAuthAdapter` implementing SDK's interface
4. Adapter delegates to CLI's `pkg/auth` functions
5. SDK receives interface, doesn't know about CLI implementation

## Implementation Steps

### Step 1: CLI Changes ✅ DONE

**Repository:** `franklouwers/stackit-cli`
**Branch:** `claude/terraform-provider-login-015Z7hLQUGxJDbBdv9HKYEhT`

The CLI provides:
- `pkg/auth.IsProviderAuthenticated()` - Check for credentials
- `pkg/auth.ProviderAuthFlow()` - Get authenticated RoundTripper
- Automatic token refresh on every HTTP request
- Thread-safe concurrent access
- Credential storage (keyring + file fallback)

**Status:** ✅ Complete

---

### Step 2: SDK Changes ⚠️ TODO

**Repository:** `stackit-sdk-go` (needs fork)
**What:** Add `CLIAuthProvider` interface (NO CLI dependency)

#### 2.1 Define Interface

**File:** `core/config/cli_auth.go`

```go
package config

import (
    "net/http"
)

// CLIAuthProvider is an interface for CLI authentication providers.
// Implementations should bridge to the STACKIT CLI without the SDK
// needing to import the CLI package directly.
//
// This interface-based approach avoids circular dependencies:
// - SDK defines the interface
// - Consumer (e.g., Terraform Provider) implements it
// - Consumer imports both SDK and CLI
// - SDK never imports CLI
type CLIAuthProvider interface {
    // IsAuthenticated checks if CLI provider credentials exist
    IsAuthenticated() bool

    // GetAuthFlow returns an authenticated http.RoundTripper
    // The RoundTripper should handle token refresh automatically
    GetAuthFlow() (http.RoundTripper, error)
}

// WithCLIProviderAuth configures authentication using a CLI auth provider.
// The provider parameter should implement the CLIAuthProvider interface.
//
// Example usage (in Terraform Provider):
//   adapter := NewCLIAuthAdapter() // implements CLIAuthProvider
//   config.WithCLIProviderAuth(adapter)(sdkConfig)
func WithCLIProviderAuth(provider CLIAuthProvider) ConfigurationOption {
    return func(c *Configuration) error {
        if provider == nil {
            return &AuthenticationError{
                Message: "CLI auth provider cannot be nil",
            }
        }

        // Get the authenticated RoundTripper from the provider
        authFlow, err := provider.GetAuthFlow()
        if err != nil {
            return &AuthenticationError{
                Message: "Failed to initialize CLI authentication",
                Err:     err,
            }
        }

        // Use the CLI's RoundTripper as our custom auth
        return WithCustomAuth(authFlow)(c)
    }
}

// AuthenticationError represents an error during authentication setup
type AuthenticationError struct {
    Message string
    Err     error
}

func (e *AuthenticationError) Error() string {
    if e.Err != nil {
        return e.Message + ": " + e.Err.Error()
    }
    return e.Message
}

func (e *AuthenticationError) Unwrap() error {
    return e.Err
}
```

**Key Points:**
- ✅ SDK defines interface only
- ✅ NO import of CLI package
- ✅ No circular dependency
- ✅ Consumer implements interface

#### 2.2 Add Tests

**File:** `core/config/cli_auth_test.go`

```go
package config

import (
    "net/http"
    "testing"
)

// mockCLIAuthProvider is a mock implementation for testing
type mockCLIAuthProvider struct {
    authenticated bool
    err           error
}

func (m *mockCLIAuthProvider) IsAuthenticated() bool {
    return m.authenticated
}

func (m *mockCLIAuthProvider) GetAuthFlow() (http.RoundTripper, error) {
    if m.err != nil {
        return nil, m.err
    }
    return http.DefaultTransport, nil
}

func TestWithCLIProviderAuth_Nil(t *testing.T) {
    config := &Configuration{}
    err := WithCLIProviderAuth(nil)(config)

    if err == nil {
        t.Error("Expected error with nil provider")
    }
}

func TestWithCLIProviderAuth_Success(t *testing.T) {
    mock := &mockCLIAuthProvider{authenticated: true}
    config := &Configuration{}

    err := WithCLIProviderAuth(mock)(config)
    if err != nil {
        t.Errorf("Unexpected error: %v", err)
    }

    if config.RoundTripper == nil {
        t.Error("Expected RoundTripper to be set")
    }
}

func TestWithCLIProviderAuth_Error(t *testing.T) {
    mock := &mockCLIAuthProvider{
        authenticated: true,
        err:           fmt.Errorf("auth failed"),
    }
    config := &Configuration{}

    err := WithCLIProviderAuth(mock)(config)
    if err == nil {
        t.Error("Expected error from auth flow")
    }
}
```

#### 2.3 Update Documentation

**File:** `README.md` or `docs/authentication.md`

```markdown
## CLI Provider Authentication

The SDK supports CLI authentication through the `CLIAuthProvider` interface.

### Interface

```go
type CLIAuthProvider interface {
    IsAuthenticated() bool
    GetAuthFlow() (http.RoundTripper, error)
}
```

Consumers (like Terraform Provider) implement this interface to bridge
to the STACKIT CLI without the SDK needing to import the CLI.

### Usage in Consumers

```go
// Consumer implements the interface
type CLIAuthAdapter struct {}

func (a *CLIAuthAdapter) IsAuthenticated() bool {
    return cliAuth.IsProviderAuthenticated()
}

func (a *CLIAuthAdapter) GetAuthFlow() (http.RoundTripper, error) {
    return cliAuth.ProviderAuthFlow(nil)
}

// Use with SDK
adapter := &CLIAuthAdapter{}
client, err := dns.NewAPIClient(
    config.WithCLIProviderAuth(adapter),
)
```

See Terraform Provider implementation for complete example.
```

**Status:** ⚠️ Needs implementation

---

### Step 3: Terraform Provider Changes ✅ DONE

**Repository:** `franklouwers/terraform-provider-stackit`
**Branch:** `claude/external-app-auth-01TKT87AQZgap9RZEczhgXaD`

#### What Was Implemented:

1. **CLI Auth Adapter** (`stackit/internal/core/cli_auth_adapter.go`)
   ```go
   package core

   import (
       "net/http"
       cliAuth "github.com/stackitcloud/stackit-cli/pkg/auth"
   )

   // CLIAuthAdapter implements SDK's CLIAuthProvider interface
   type CLIAuthAdapter struct{}

   func NewCLIAuthAdapter() *CLIAuthAdapter {
       return &CLIAuthAdapter{}
   }

   func (a *CLIAuthAdapter) IsAuthenticated() bool {
       return cliAuth.IsProviderAuthenticated()
   }

   func (a *CLIAuthAdapter) GetAuthFlow() (http.RoundTripper, error) {
       return cliAuth.ProviderAuthFlow(nil)
   }
   ```

2. **Added `cli_auth` attribute** (`stackit/provider.go:162`)
   ```hcl
   provider "stackit" {
     cli_auth = true  # Explicit opt-in
   }
   ```

3. **Authentication logic** (`stackit/provider.go:477-493`)
   ```go
   if !hasExplicitAuth && cliAuthEnabled {
       adapter := core.NewCLIAuthAdapter()

       if !adapter.IsAuthenticated() {
           return Error("Please run 'stackit auth provider login'")
       }

       err = config.WithCLIProviderAuth(adapter)(sdkConfig)
   }
   ```

4. **Priority order:**
   1. Explicit credentials (service_account_key, token)
   2. CLI auth (when `cli_auth = true`)
   3. Environment variables / credentials file

**Key Design Decisions:**
- ✅ Adapter pattern avoids circular dependencies
- ✅ Provider imports both CLI and SDK
- ✅ SDK never imports CLI
- ✅ Clear error messages
- ✅ Explicit opt-in required

**Status:** ✅ Complete (pending SDK interface support)

---

## Testing Strategy

### Unit Tests (SDK)

```bash
# In stackit-sdk-go repository
cd core/config
go test -v -run TestWithCLIProviderAuth
```

### Integration Tests (Terraform Provider)

**Setup:**
```bash
# Authenticate with CLI
stackit auth provider login

# Export test credentials
export STACKIT_PROJECT_ID="your-project-id"
```

**Test scenarios:**

1. **CLI auth enabled with valid credentials:**
   ```hcl
   provider "stackit" {
     cli_auth = true
   }

   resource "stackit_dns_zone" "test" {
     # Should use CLI credentials
   }
   ```

2. **CLI auth enabled but no credentials:**
   ```hcl
   provider "stackit" {
     cli_auth = true  # Should fail with clear error
   }
   ```

3. **Explicit credentials override CLI auth:**
   ```hcl
   provider "stackit" {
     cli_auth              = true
     service_account_key   = "explicit_key"  # Takes precedence
   }
   ```

4. **CLI auth disabled (default):**
   ```hcl
   provider "stackit" {
     # cli_auth not set - should NOT check CLI credentials
   }
   ```

---

## Deployment Steps

### For Development/Testing:

1. **Fork and update SDK:**
   ```bash
   git clone https://github.com/stackitcloud/stackit-sdk-go
   cd stackit-sdk-go
   git checkout -b feature/cli-auth

   # Add the changes from Step 2
   # Commit and push to your fork
   ```

2. **Update Terraform Provider go.mod:**
   ```go
   // Uncomment and update in go.mod:
   replace github.com/stackitcloud/stackit-sdk-go/core => github.com/YOUR-USERNAME/stackit-sdk-go/core v0.0.0-COMMIT-HASH
   ```

3. **Build and test:**
   ```bash
   cd terraform-provider-stackit
   go mod tidy
   go build

   # Run tests
   make test
   make testacc
   ```

### For Production:

1. Submit PR to `stackitcloud/stackit-sdk-go` with CLI auth changes
2. Wait for SDK release with CLI auth support
3. Update terraform-provider to use new SDK version
4. Submit PR to `stackitcloud/terraform-provider-stackit`

---

## Benefits of This Architecture

### ✅ No Circular Dependencies
- SDK defines interface, doesn't import CLI
- Provider implements interface and imports both
- Clean dependency graph:
  - CLI: No dependencies on SDK or Provider
  - SDK: No dependencies on CLI or Provider
  - Provider: Depends on both CLI and SDK

### ✅ Proper Layering
- CLI provides auth primitives
- SDK provides interface contract
- Provider bridges the two

### ✅ Reusability
- Any SDK user can implement `CLIAuthProvider`
- Not limited to Terraform
- Works with any Go application
- Interface can support multiple CLI implementations

### ✅ Maintainability
- CLI changes don't break SDK
- SDK changes to interface are backwards compatible
- Provider adapter is ~15 lines of code
- Clear separation of concerns

### ✅ Testability
- SDK can be tested with mock implementations
- Provider adapter is trivial to test
- No need for CLI in SDK tests
- Integration tests can use real CLI

---

## Troubleshooting

### Error: "CLI provider authentication not found"

**Solution:**
```bash
stackit auth provider login
```

### Error: "Failed to initialize CLI authentication"

**Possible causes:**
- Credentials expired or invalid
- Keyring access denied
- Storage file corrupted

**Solution:**
```bash
stackit auth provider logout
stackit auth provider login
```

### Provider ignores CLI credentials

**Check:**
1. Is `cli_auth = true` set in provider config?
2. Is SDK version >= X.X.X (with CLI auth support)?
3. Are credentials actually stored? Run: `stackit auth provider status`

---

## Migration Path

### Phase 1: Beta (Current)
- SDK changes in fork
- Provider uses forked SDK
- Testing with early adopters

### Phase 2: Integration
- Submit SDK PR
- SDK release with CLI auth
- Update provider to use released SDK

### Phase 3: General Availability
- Update documentation
- Announce feature
- Deprecate old auth methods (if applicable)

---

## Questions?

For issues or questions:
- SDK changes: Open issue in `stackit-sdk-go` repository
- Provider changes: Open issue in `terraform-provider-stackit` repository
- CLI changes: Open issue in `stackit-cli` repository
