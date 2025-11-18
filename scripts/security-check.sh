#!/bin/bash

# Enhanced Local Connector - Security Check Script
# This script performs automated security checks on the codebase

set -e

echo "=========================================="
echo "Enhanced Local Connector - Security Check"
echo "=========================================="
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

# Color codes
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

ISSUES_FOUND=0
WARNINGS_FOUND=0

# Helper functions
fail() {
    echo -e "${RED}✗ FAIL:${NC} $1"
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
}

warn() {
    echo -e "${YELLOW}⚠ WARN:${NC} $1"
    WARNINGS_FOUND=$((WARNINGS_FOUND + 1))
}

pass() {
    echo -e "${GREEN}✓ PASS:${NC} $1"
}

section() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

# Check 1: File Permissions
section "1. File Permissions Check"

echo "Checking storage file permissions..."

# Check if storage directory exists
if [ -d "data" ]; then
    # Check for world-readable files
    WORLD_READABLE=$(find data -type f -perm -004 2>/dev/null || true)
    if [ -n "$WORLD_READABLE" ]; then
        fail "World-readable files found in data/ directory"
        echo "$WORLD_READABLE"
    else
        pass "No world-readable files in data/ directory"
    fi

    # Check for group-readable files
    GROUP_READABLE=$(find data -type f -perm -040 2>/dev/null || true)
    if [ -n "$GROUP_READABLE" ]; then
        warn "Group-readable files found in data/ directory (should be 0600)"
        echo "$GROUP_READABLE"
    else
        pass "File permissions are properly restrictive (0600)"
    fi
else
    warn "data/ directory not found (will be created on first run)"
fi

# Check 2: Hardcoded Secrets
section "2. Hardcoded Secrets Check"

echo "Scanning for potential hardcoded secrets..."

# Patterns to search for
PATTERNS=(
    "password.*=.*['\"][^'\"]{8,}['\"]"
    "secret.*=.*['\"][^'\"]{16,}['\"]"
    "api[_-]?key.*=.*['\"][^'\"]{16,}['\"]"
    "token.*=.*['\"][^'\"]{20,}['\"]"
)

SECRET_FOUND=0
for pattern in "${PATTERNS[@]}"; do
    MATCHES=$(grep -r -i -E "$pattern" \
        --include="*.go" \
        --exclude-dir=vendor \
        --exclude-dir=.git \
        connector/local-enhanced/ 2>/dev/null || true)

    if [ -n "$MATCHES" ]; then
        SECRET_FOUND=1
        echo "$MATCHES"
    fi
done

if [ $SECRET_FOUND -eq 1 ]; then
    fail "Potential hardcoded secrets found (review manually)"
else
    pass "No obvious hardcoded secrets found"
fi

# Check 3: HTTPS Validation
section "3. HTTPS Configuration Check"

echo "Checking for HTTPS validation..."

# Check if RPOrigins validation exists
if grep -q "HasPrefix.*https://" connector/local-enhanced/config.go; then
    pass "HTTPS validation found in config"
else
    fail "Missing HTTPS validation for RPOrigins in config.go"
    echo "  Recommendation: Add validation in Config.Validate() to ensure RPOrigins use HTTPS"
fi

# Check for insecure HTTP usage
HTTP_USAGE=$(grep -r "http://" \
    --include="*.go" \
    --exclude-dir=vendor \
    --exclude-dir=.git \
    connector/local-enhanced/ | \
    grep -v "localhost" | \
    grep -v "127.0.0.1" | \
    grep -v "// " | \
    grep -v "//" || true)

if [ -n "$HTTP_USAGE" ]; then
    warn "Insecure HTTP URLs found (excluding localhost)"
    echo "$HTTP_USAGE"
else
    pass "No insecure HTTP URLs found (excluding localhost)"
fi

# Check 4: Rate Limiting
section "4. Rate Limiting Check"

echo "Checking rate limiting implementation..."

# Check TOTP rate limiting
if grep -q "TOTPRateLimiter" connector/local-enhanced/totp.go; then
    pass "TOTP rate limiting implemented"
else
    fail "Missing TOTP rate limiting"
fi

# Check Magic Link rate limiting
if grep -q "MagicLinkRateLimiter" connector/local-enhanced/magiclink.go; then
    pass "Magic link rate limiting implemented"
else
    fail "Missing magic link rate limiting"
fi

# Check Password rate limiting
if grep -q "PasswordRateLimiter" connector/local-enhanced/password.go; then
    pass "Password rate limiting implemented"
else
    fail "Missing password rate limiting (CRITICAL)"
    echo "  Recommendation: Implement PasswordRateLimiter similar to TOTPRateLimiter"
fi

# Check 5: Constant-Time Comparisons
section "5. Constant-Time Comparison Check"

echo "Checking for timing-safe comparisons..."

# Check for bcrypt usage (constant-time)
if grep -q "bcrypt.CompareHashAndPassword" connector/local-enhanced/password.go; then
    pass "Password verification uses bcrypt (constant-time)"
else
    warn "Password verification may not use constant-time comparison"
fi

# Check for subtle.ConstantTimeCompare usage
SUBTLE_USAGE=$(grep -r "subtle.ConstantTimeCompare" \
    --include="*.go" \
    connector/local-enhanced/ || true)

if [ -n "$SUBTLE_USAGE" ]; then
    pass "subtle.ConstantTimeCompare found in codebase"
else
    warn "No usage of subtle.ConstantTimeCompare (consider for token comparisons)"
fi

# Check for direct string comparison of secrets
DIRECT_COMPARE=$(grep -r "==.*token\|==.*secret\|==.*password" \
    --include="*.go" \
    connector/local-enhanced/ | \
    grep -v "// " | \
    grep -v "//" || true)

if [ -n "$DIRECT_COMPARE" ]; then
    warn "Direct string comparison of sensitive data found (potential timing attack)"
    echo "$DIRECT_COMPARE" | head -5
else
    pass "No obvious direct comparisons of sensitive data"
fi

# Check 6: Input Validation
section "6. Input Validation Check"

echo "Checking input validation functions..."

# Check for validation functions
if grep -q "ValidateEmail" connector/local-enhanced/validation.go; then
    pass "Email validation implemented"
else
    fail "Missing email validation function"
fi

if grep -q "ValidatePassword" connector/local-enhanced/validation.go; then
    pass "Password validation implemented"
else
    fail "Missing password validation function"
fi

if grep -q "ValidateUsername" connector/local-enhanced/validation.go; then
    pass "Username validation implemented"
else
    fail "Missing username validation function"
fi

# Check if validation is actually used in handlers
VALIDATION_USAGE=$(grep -r "Validate.*(" \
    --include="*.go" \
    connector/local-enhanced/handlers.go | \
    wc -l)

if [ "$VALIDATION_USAGE" -gt 5 ]; then
    pass "Input validation is used in handlers ($VALIDATION_USAGE occurrences)"
else
    warn "Input validation may not be consistently used in handlers"
fi

# Check 7: Error Message Analysis
section "7. Error Message Information Leakage"

echo "Checking for information disclosure in error messages..."

# Check for "user not found" messages
USER_NOT_FOUND=$(grep -r "User not found\|user not found" \
    --include="*.go" \
    connector/local-enhanced/ || true)

if [ -n "$USER_NOT_FOUND" ]; then
    warn "Error messages may allow user enumeration"
    echo "$USER_NOT_FOUND" | head -3
    echo "  Recommendation: Use generic 'Authentication failed' instead"
else
    pass "No obvious user enumeration vulnerabilities"
fi

# Check for overly specific error messages
SPECIFIC_ERRORS=$(grep -r "already enabled\|already exists\|already in use" \
    --include="*.go" \
    connector/local-enhanced/ || true)

if [ -n "$SPECIFIC_ERRORS" ]; then
    warn "Overly specific error messages found (may leak information)"
    echo "$SPECIFIC_ERRORS" | head -3
else
    pass "No overly specific error messages found"
fi

# Check 8: Crypto Package Usage
section "8. Cryptography Check"

echo "Checking cryptographic library usage..."

# Check for crypto/rand usage (secure random)
if grep -q "crypto/rand" connector/local-enhanced/*.go; then
    pass "crypto/rand used for random number generation"
else
    fail "Missing crypto/rand import (may use insecure math/rand)"
fi

# Check for math/rand usage (insecure for crypto)
MATH_RAND=$(grep -r "math/rand" \
    --include="*.go" \
    connector/local-enhanced/ || true)

if [ -n "$MATH_RAND" ]; then
    fail "math/rand used (insecure for cryptographic purposes)"
    echo "$MATH_RAND"
else
    pass "No usage of insecure math/rand"
fi

# Check for weak hashing algorithms
WEAK_HASH=$(grep -r "md5\|sha1\.New()" \
    --include="*.go" \
    connector/local-enhanced/ || true)

if [ -n "$WEAK_HASH" ]; then
    fail "Weak hashing algorithms (MD5/SHA1) found"
    echo "$WEAK_HASH"
else
    pass "No weak hashing algorithms found"
fi

# Check 9: Dependency Vulnerabilities
section "9. Dependency Security Check"

echo "Checking Go module dependencies for known vulnerabilities..."

if command -v govulncheck &> /dev/null; then
    echo "Running govulncheck..."
    govulncheck ./... || warn "Vulnerabilities found in dependencies"
else
    warn "govulncheck not installed (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)"
fi

# Check 10: Configuration Security
section "10. Configuration Security Check"

echo "Checking configuration security..."

# Check for example config files with defaults
if [ -f "config.dev.yaml" ]; then
    if grep -q "changeme\|example\|test123" config.dev.yaml; then
        warn "Example passwords found in config.dev.yaml"
    else
        pass "No obvious default passwords in dev config"
    fi
fi

# Check for .env files that should not be committed
if [ -f ".env" ]; then
    warn ".env file found (should be in .gitignore)"
else
    pass "No .env file in repository"
fi

# Check if .env is in .gitignore
if [ -f ".gitignore" ]; then
    if grep -q "\.env" .gitignore; then
        pass ".env is in .gitignore"
    else
        warn ".env not found in .gitignore"
    fi
fi

# Summary
section "Security Check Summary"

echo ""
if [ $ISSUES_FOUND -eq 0 ] && [ $WARNINGS_FOUND -eq 0 ]; then
    echo -e "${GREEN}✓ All security checks passed!${NC}"
    echo ""
    exit 0
elif [ $ISSUES_FOUND -eq 0 ]; then
    echo -e "${YELLOW}⚠ Security checks completed with $WARNINGS_FOUND warnings${NC}"
    echo ""
    echo "Warnings should be reviewed but are not critical."
    exit 0
else
    echo -e "${RED}✗ Security checks completed with $ISSUES_FOUND issues and $WARNINGS_FOUND warnings${NC}"
    echo ""
    echo "Critical issues found. Please review the security audit report:"
    echo "  docs/enhancements/security-audit.md"
    echo ""
    exit 1
fi
