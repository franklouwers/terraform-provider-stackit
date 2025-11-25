# CLI Provider Authentication - Implementation Guide

This guide explains how to implement CLI provider authentication following the proposal architecture.

## Architecture Overview

```
┌─────────────┐     ┌──────────────┐     ┌────────────────────┐
│  STACKIT    │────▶│  STACKIT     │────▶│   Terraform        │
│  CLI        │     │  SDK         │     │   Provider         │
│  (pkg/auth) │     │  (WithCLI..) │     │   (cli_auth=true)  │
└─────────────┘     └──────────────┘     └────────────────────┘
```

**Flow:**
1. User runs: `stackit auth provider login`
2. CLI stores credentials (keyring or file)
3. SDK provides `WithCLIProviderAuth()` wrapper
4. Terraform Provider uses SDK's CLI auth option

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
**What:** Add CLI authentication as standard auth method

#### 2.1 Add CLI Dependency

**File:** `go.mod`

```go
require (
    github.com/stackitcloud/stackit-cli v0.0.0-20251125162153-bfebe445230c
    // ... other dependencies
)

replace github.com/stackitcloud/stackit-cli => github.com/franklouwers/stackit-cli v0.0.0-20251125162153-bfebe445230c
```

#### 2.2 Create CLI Auth Module

**File:** `core/config/cli_auth.go`

```go
package config

import (
    "net/http"

    cliAuth "github.com/stackitcloud/stackit-cli/pkg/auth"
)

// WithCLIProviderAuth configures authentication using STACKIT CLI provider credentials.
// This uses credentials from 'stackit auth provider login' command.
//
// Returns an error if:
// - CLI provider credentials are not found (user hasn't run 'stackit auth provider login')
// - Failed to initialize the authentication flow
//
// The CLI handles automatic token refresh and credential storage.
func WithCLIProviderAuth() ConfigurationOption {
    return func(c *Configuration) error {
        if !cliAuth.IsProviderAuthenticated() {
            return &AuthenticationError{
                Message: "CLI provider authentication not found. Please run 'stackit auth provider login' first.",
            }
        }

        authFlow, err := cliAuth.ProviderAuthFlow(nil)
        if err != nil {
            return &AuthenticationError{
                Message: "Failed to initialize CLI authentication",
                Err:     err,
            }
        }

        return WithCustomAuth(authFlow)(c)
    }
}

// IsCLIProviderAuthenticated checks if CLI provider credentials are available.
func IsCLIProviderAuthenticated() bool {
    return cliAuth.IsProviderAuthenticated()
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

#### 2.3 Add Tests

**File:** `core/config/cli_auth_test.go`

```go
package config

import "testing"

func TestWithCLIProviderAuth_NotAuthenticated(t *testing.T) {
    config := &Configuration{}
    err := WithCLIProviderAuth()(config)

    if err == nil {
        t.Error("Expected error when CLI provider not authenticated")
    }
}

func TestIsCLIProviderAuthenticated(t *testing.T) {
    isAuth := IsCLIProviderAuthenticated()
    t.Logf("CLI Provider Authenticated: %v", isAuth)
}

func TestWithCLIProviderAuth_Integration(t *testing.T) {
    if !IsCLIProviderAuthenticated() {
        t.Skip("Skipping: CLI provider not authenticated")
    }

    config := &Configuration{}
    err := WithCLIProviderAuth()(config)

    if err != nil {
        t.Errorf("Failed to configure CLI auth: %v", err)
    }

    if config.RoundTripper == nil {
        t.Error("Expected RoundTripper to be set")
    }
}
```

#### 2.4 Update Documentation

**File:** `README.md` or `docs/authentication.md`

```markdown
## CLI Provider Authentication

Authenticate using STACKIT CLI credentials:

```go
import (
    "github.com/stackitcloud/stackit-sdk-go/core/config"
    "github.com/stackitcloud/stackit-sdk-go/services/dns"
)

func main() {
    // Uses credentials from 'stackit auth provider login'
    client, err := dns.NewAPIClient(
        config.WithCLIProviderAuth(),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

Prerequisites:
- Run `stackit auth provider login` first
- CLI handles automatic token refresh
```

**Status:** ⚠️ Needs implementation

---

### Step 3: Terraform Provider Changes ✅ DONE

**Repository:** `franklouwers/terraform-provider-stackit`
**Branch:** `claude/external-app-auth-01TKT87AQZgap9RZEczhgXaD`

#### What Was Implemented:

1. **Added `cli_auth` attribute** (`stackit/provider.go:163`)
   ```hcl
   provider "stackit" {
     cli_auth = true  # Explicit opt-in
   }
   ```

2. **Authentication logic** (`stackit/provider.go:477-493`)
   - Checks `cli_auth = true`
   - Applies SDK's `config.WithCLIProviderAuth()`
   - Falls back to traditional auth if CLI auth not enabled

3. **Priority order:**
   1. Explicit credentials (service_account_key, token)
   2. CLI auth (when `cli_auth = true`)
   3. Environment variables / credentials file

**Status:** ✅ Complete (pending SDK support)

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

### ✅ Proper Layering
- CLI provides auth primitives
- SDK provides auth integration
- Provider uses SDK's standard API

### ✅ Reusability
- Any SDK user can use `WithCLIProviderAuth()`
- Not limited to Terraform
- Go applications, scripts, etc.

### ✅ Maintainability
- CLI changes don't affect Provider
- SDK abstracts CLI implementation details
- Clear separation of concerns

### ✅ Testability
- Each layer can be tested independently
- SDK can mock CLI auth
- Provider tests use SDK mocks

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
