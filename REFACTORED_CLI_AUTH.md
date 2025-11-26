# CLI Authentication Refactored - No More Dependency Conflicts!

## Problem Solved ✅

The original approach created a **diamond dependency**:
```
Provider → CLI → SDK v0.X.X
Provider → SDK v0.Y.Y
```
This caused impossible-to-resolve version conflicts between the CLI's SDK version and the Provider's SDK version.

## New Architecture: File-Based Credential Sharing

The CLI and Provider now communicate through a **shared credential file**, similar to AWS CLI and Terraform:

```
CLI → SDK (stores credentials to ~/.stackit/provider-credentials.json)
Provider → SDK (reads ~/.stackit/provider-credentials.json)
```

**Zero code dependencies** between CLI and Provider!

## Changes Made

### 1. Removed CLI Dependency (`go.mod`)
- ❌ Removed: `github.com/stackitcloud/stackit-cli` dependency
- ❌ Removed: CLI fork replace directive
- ❌ Removed: SDK fork replace directive (no longer needed)
- ✅ Provider now depends ONLY on the standard SDK (no forks!)

### 2. New Credential Reader (`stackit/internal/core/cli_credentials.go`)

Simple file-based credential reader:
- Reads from `~/.stackit/provider-credentials.json` (or `$STACKIT_CLI_CONFIG_DIR/provider-credentials.json`)
- Parses OAuth credentials (access_token, refresh_token, expiry)
- Validates credentials exist before use

```go
type ProviderCredentials struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    Expiry       time.Time `json:"expiry"`
    TokenType    string    `json:"token_type,omitempty"`
}
```

### 3. Updated Provider Logic (`stackit/provider.go`)

Simplified authentication flow:
```go
if !hasExplicitAuth && cliAuthEnabled {
    // Read credentials from file (no CLI code dependency!)
    creds, err := core.ReadCLICredentials()
    if err != nil {
        // Error: CLI credentials not found
        return
    }

    // Use token directly with SDK
    sdkConfig.Token = creds.AccessToken
}
```

### 4. Removed Old Adapter
- ❌ Deleted: `stackit/internal/core/cli_auth_adapter.go` (no longer needed)

## How It Works

1. **User authenticates via CLI:**
   ```bash
   stackit auth provider login
   ```
   → CLI performs OAuth flow and stores credentials to `~/.stackit/provider-credentials.json`

2. **Provider reads credentials:**
   ```hcl
   provider "stackit" {
     cli_auth = true  # Enable CLI authentication
   }
   ```
   → Provider reads token from file and uses it with SDK

3. **No version conflicts!**
   - CLI can use any SDK version it needs
   - Provider can use any SDK version it needs
   - They only share a JSON file format

## Token Refresh

**Simple approach:** Token refresh is handled by the CLI
- When token expires, user runs: `stackit auth provider login` again
- This is similar to how `aws sso login` works
- Provider always uses the current token from file

**Future enhancement:** Provider could implement automatic refresh by:
1. Checking token expiry
2. Using refresh_token to get new access_token
3. Writing updated credentials back to file

But for MVP, manual refresh via CLI is simpler and proven!

## Testing

Once network access is restored, test with:
```bash
# Clean dependencies
go mod tidy

# Build provider
go build

# Run tests
go test ./...
```

## Next Steps for CLI

The CLI needs to store credentials in the agreed-upon format:

**Location:** `~/.stackit/provider-credentials.json` (or `$STACKIT_CLI_CONFIG_DIR/provider-credentials.json`)

**Format:**
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "expiry": "2025-11-27T10:30:00Z",
  "token_type": "Bearer"
}
```

The CLI's `stackit auth provider login` command should:
1. Perform OAuth flow (already implemented in your fork)
2. Write credentials to the file above in JSON format
3. Set appropriate file permissions (0600 - owner read/write only)

## Benefits

✅ **No version conflicts** - CLI and Provider are independent
✅ **No circular dependencies** - Clean separation
✅ **No SDK forks needed** - Use standard released versions
✅ **Standard pattern** - Same as AWS/Azure/GCP CLIs
✅ **Simpler testing** - No complex dependency graphs
✅ **Easier maintenance** - Changes to CLI don't affect Provider

## Comparison to AWS CLI

This is exactly how AWS does it:
- AWS CLI: `aws sso login` → Writes to `~/.aws/sso/cache/`
- Terraform AWS Provider: Reads from `~/.aws/sso/cache/`
- No code dependency between aws-cli and terraform-provider-aws!
