# Changelog

All notable changes to the Dex Enhanced Local Connector will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Testing Infrastructure** (2025-11-18)
  - Comprehensive testing utilities in `connector/local-enhanced/testing.go`
  - Test helpers for creating test users, passkeys, sessions, and tokens
  - Mock objects for external dependencies (email sender)
  - Assertion helpers for file operations and permissions
  - Make targets for running tests with coverage and race detection

- **Dependencies** (2025-11-17)
  - `github.com/go-webauthn/webauthn` v0.11.2 for WebAuthn/passkey support
  - `github.com/pquerna/otp` v1.4.0 for TOTP 2FA
  - `github.com/golang-jwt/jwt/v5` v5.2.1 for JWT token generation (magic links)
  - `github.com/skip2/go-qrcode` v0.0.0-20200617195104 for QR code generation

- **Documentation** (2025-11-18)
  - DEVELOPMENT.md with comprehensive development environment setup guide
  - CODING_STANDARDS.md with detailed coding conventions and best practices
  - CHANGELOG.md for tracking project changes
  - Testing documentation in CLAUDE.md
  - Implementation plan in TODO.md

- **Project Setup** (2025-11-17)
  - Feature branch `feature/passkeys` created
  - Nix flake configuration for reproducible development environment
  - Make targets for testing: `test-local-enhanced`, `test-local-enhanced-coverage`, `test-local-enhanced-race`

### Changed
- None

### Fixed
- None

### Security
- None

### Deprecated
- None

### Removed
- None

---

## [0.1.0] - 2025-11-17

### Added
- **File-based Storage Backend**
  - JSON file storage for users and credentials
  - Individual files per user (one file per user ID)
  - Deterministic user IDs (SHA-256 of email)
  - No database required
  - Location: `storage/file/`

### Changed
- Forked from dexidp/dex for Enopax-specific enhancements

---

## Changelog Guidelines

### Categories

Use these categories to organize changes:

- **Added**: New features
- **Changed**: Changes in existing functionality
- **Fixed**: Bug fixes
- **Security**: Security fixes or improvements
- **Deprecated**: Soon-to-be removed features
- **Removed**: Removed features

### Example Entry Format

```markdown
### Added
- **Feature Name** (YYYY-MM-DD)
  - Description of what was added
  - Additional context or details
  - Reference to issue/PR if applicable
```

### When to Update

Update this changelog:

1. **After completing a task** from TODO.md
2. **Before committing** significant changes
3. **When adding new features**
4. **When fixing bugs**
5. **When making breaking changes**

### Versioning

- **Unreleased**: Work in progress
- **Patch** (0.0.x): Bug fixes, minor changes
- **Minor** (0.x.0): New features, backwards compatible
- **Major** (x.0.0): Breaking changes

---

## Past Versions

### Version History

- `[Unreleased]` - Current development
- `[0.1.0] - 2025-11-17` - Initial file-based storage implementation

---

## How to Contribute

When making changes:

1. **Add your change** to the `[Unreleased]` section
2. **Use the correct category** (Added, Changed, Fixed, etc.)
3. **Include the date** in YYYY-MM-DD format
4. **Be descriptive** - explain what and why
5. **Reference issues** if applicable

Example commit that updates changelog:

```bash
git add CHANGELOG.md
git add feature/new-feature.go
git commit -m "feat(passkey): add passkey registration endpoint

Added POST /auth/passkey/register/begin endpoint.

Updates:
- CHANGELOG.md: Added passkey registration feature
- TODO.md: Marked Phase 2 Week 6 task complete
"
```

---

## Links

- [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)
- [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
- [Project TODO](./TODO.md)
- [Development Guide](./DEVELOPMENT.md)
- [Coding Standards](./CODING_STANDARDS.md)

---

**Last Updated**: 2025-11-18
**Maintainer**: Enopax Platform Team
