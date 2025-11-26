# Create SDK Fork from v0.19.0

## ✅ Provider is now at v0.19.0

The provider has been downgraded to use SDK **core v0.19.0** - the stable version from main branch.

## Create Your SDK Fork

### 1. Clone and Checkout v0.19.0

```bash
git clone https://github.com/stackitcloud/stackit-sdk-go
cd stackit-sdk-go

# Checkout the v0.19.0 tag (stable version)
git checkout core/v0.19.0

# Create your feature branch from this tag
git checkout -b feature/cli-auth-v0.19.0
```

### 2. Add the CLI Auth Interface

Create file **`core/config/cli_auth.go`**:

```go
package config

import "net/http"

// CLIAuthProvider is an interface for CLI authentication providers.
// Implementations should bridge to the STACKIT CLI without the SDK
// needing to import the CLI package directly.
type CLIAuthProvider interface {
	IsAuthenticated() bool
	GetAuthFlow() (http.RoundTripper, error)
}

// WithCLIProviderAuth configures authentication using a CLI auth provider.
func WithCLIProviderAuth(provider CLIAuthProvider) ConfigurationOption {
	return func(c *Configuration) error {
		if provider == nil {
			return &AuthenticationError{
				Message: "CLI auth provider cannot be nil",
			}
		}

		authFlow, err := provider.GetAuthFlow()
		if err != nil {
			return &AuthenticationError{
				Message: "Failed to initialize CLI authentication",
				Err:     err,
			}
		}

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

### 3. Commit and Push

```bash
git add core/config/cli_auth.go
git commit -m "feat: add CLIAuthProvider interface for external CLI integration

Adds interface-based CLI authentication support that avoids circular
dependencies by letting consumers implement the interface.

Based on core/v0.19.0"

# Push to your fork
git remote add myfork git@github.com:YOUR-USERNAME/stackit-sdk-go.git
git push myfork feature/cli-auth-v0.19.0

# Get the commit hash (you'll need this!)
git log -1 --format="%H"
```

### 4. Update Provider's go.mod

Copy the commit hash from step 3, then update the replace directive in `terraform-provider-stackit/go.mod`:

```bash
cd terraform-provider-stackit

# Edit go.mod line 130 - replace v0.0.0-UPDATEME with your commit:
# replace github.com/stackitcloud/stackit-sdk-go/core => github.com/YOUR-USERNAME/stackit-sdk-go/core v0.19.1-0.YYYYMMDDHHMMSS-COMMITHASH

# Example:
# replace github.com/stackitcloud/stackit-sdk-go/core => github.com/franklouwers/stackit-sdk-go/core v0.19.1-0.20251126120000-abc123def456
```

**Version format:**
- `v0.19.1-0` - Increment patch from v0.19.0
- `YYYYMMDDHHMMSS` - Timestamp (from `git log --format=%ct <commit> | xargs -I {} date -u -d @{} +%Y%m%d%H%M%S`)
- `COMMITHASH` - First 12 characters of commit hash

### 5. Test

```bash
go mod tidy
go build

# Should build successfully!
```

## Why v0.19.0?

✅ **Stable** - Used by main branch, battle-tested
✅ **Known good** - No unknown breaking changes
✅ **Minimal risk** - Only your CLI auth changes
✅ **Easy to debug** - If it breaks, it's your change

## Quick Commands

```bash
# In stackit-sdk-go repo:
git checkout core/v0.19.0
git checkout -b feature/cli-auth-v0.19.0
# Add core/config/cli_auth.go (copy from above)
git add core/config/cli_auth.go
git commit -m "feat: add CLIAuthProvider interface"
git push myfork feature/cli-auth-v0.19.0
git log -1 --format="%H"  # Copy this hash!

# In terraform-provider-stackit repo:
# Update go.mod line 130 with your fork and commit hash
go mod tidy
go build
```

## That's It!

Once your SDK fork is created and go.mod is updated, the provider should build successfully with CLI auth support.
