package local

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Validate validates a magic link token.
func (t *MagicLinkToken) Validate() error {
	if t.Token == "" {
		return fmt.Errorf("token is required")
	}
	if t.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if err := ValidateEmail(t.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}
	if t.CallbackURL == "" {
		return fmt.Errorf("callback_url is required")
	}
	if t.State == "" {
		return fmt.Errorf("state is required")
	}
	if t.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	if t.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	if t.ExpiresAt.Before(t.CreatedAt) {
		return fmt.Errorf("expires_at must be after created_at")
	}
	return nil
}

// IsExpired checks if the token has expired.
func (t *MagicLinkToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// MagicLinkRateLimiter implements rate limiting for magic link requests.
type MagicLinkRateLimiter struct {
	mu sync.Mutex

	// attempts tracks attempts by email address
	attempts map[string][]time.Time

	// hourlyLimit is the maximum attempts per hour
	hourlyLimit int

	// dailyLimit is the maximum attempts per day
	dailyLimit int
}

// NewMagicLinkRateLimiter creates a new rate limiter.
func NewMagicLinkRateLimiter(hourlyLimit, dailyLimit int) *MagicLinkRateLimiter {
	return &MagicLinkRateLimiter{
		attempts:    make(map[string][]time.Time),
		hourlyLimit: hourlyLimit,
		dailyLimit:  dailyLimit,
	}
}

// Allow checks if a request should be allowed.
func (r *MagicLinkRateLimiter) Allow(email string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	oneDayAgo := now.Add(-24 * time.Hour)

	// Get attempts for this email
	attempts := r.attempts[email]

	// Filter out attempts older than 1 day
	var recentAttempts []time.Time
	for _, t := range attempts {
		if t.After(oneDayAgo) {
			recentAttempts = append(recentAttempts, t)
		}
	}

	// Count attempts in the last hour and day
	var hourlyCount, dailyCount int
	for _, t := range recentAttempts {
		if t.After(oneHourAgo) {
			hourlyCount++
		}
		dailyCount++
	}

	// Check limits
	if hourlyCount >= r.hourlyLimit {
		return false
	}
	if dailyCount >= r.dailyLimit {
		return false
	}

	// Allow the request and record the attempt
	recentAttempts = append(recentAttempts, now)
	r.attempts[email] = recentAttempts

	return true
}

// Reset resets the rate limit for an email (e.g., after successful auth).
func (r *MagicLinkRateLimiter) Reset(email string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, email)
}

// Cleanup removes old attempts from memory.
func (r *MagicLinkRateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	oneDayAgo := time.Now().Add(-24 * time.Hour)

	for email, attempts := range r.attempts {
		var recentAttempts []time.Time
		for _, t := range attempts {
			if t.After(oneDayAgo) {
				recentAttempts = append(recentAttempts, t)
			}
		}

		if len(recentAttempts) == 0 {
			delete(r.attempts, email)
		} else {
			r.attempts[email] = recentAttempts
		}
	}
}

// generateMagicLinkToken generates a cryptographically secure token.
func generateMagicLinkToken() (string, error) {
	// Generate 32 bytes of random data
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 URL-safe string
	token := base64.URLEncoding.EncodeToString(b)
	return token, nil
}

// CreateMagicLink creates a magic link token for the given user and email.
func (c *Connector) CreateMagicLink(ctx context.Context, email, callbackURL, state, ipAddress string) (*MagicLinkToken, error) {
	// Check if magic links are enabled
	if !c.config.MagicLink.Enabled {
		return nil, fmt.Errorf("magic link authentication is disabled")
	}

	// Validate email
	if err := ValidateEmail(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Get or create user by email
	user, err := c.storage.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Generate token
	token, err := generateMagicLinkToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create magic link token
	now := time.Now()
	ttl := time.Duration(c.config.MagicLink.TTL) * time.Second
	magicLink := &MagicLinkToken{
		Token:       token,
		UserID:      user.ID,
		Email:       email,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		Used:        false,
		IPAddress:   ipAddress,
		CallbackURL: callbackURL,
		State:       state,
	}

	// Validate token
	if err := magicLink.Validate(); err != nil {
		return nil, fmt.Errorf("invalid magic link token: %w", err)
	}

	// Save token
	if err := c.storage.SaveMagicLinkToken(ctx, magicLink); err != nil {
		return nil, fmt.Errorf("failed to save magic link token: %w", err)
	}

	c.logger.Infof("Created magic link token for user %s (email: %s)", user.ID, email)
	return magicLink, nil
}

// VerifyMagicLink verifies a magic link token and marks it as used.
func (c *Connector) VerifyMagicLink(ctx context.Context, token string) (*User, string, string, error) {
	// Get token from storage
	magicLink, err := c.storage.GetMagicLinkToken(ctx, token)
	if err != nil {
		c.logger.Errorf("Magic link token not found: %v", err)
		return nil, "", "", fmt.Errorf("invalid or expired magic link")
	}

	// Check if token has expired
	if magicLink.IsExpired() {
		c.logger.Warnf("Magic link token expired for user %s", magicLink.UserID)
		return nil, "", "", fmt.Errorf("magic link has expired")
	}

	// Check if token has already been used
	if magicLink.Used {
		c.logger.Warnf("Magic link token already used for user %s", magicLink.UserID)
		return nil, "", "", fmt.Errorf("magic link has already been used")
	}

	// Get user
	user, err := c.storage.GetUser(ctx, magicLink.UserID)
	if err != nil {
		c.logger.Errorf("User not found for magic link: %v", err)
		return nil, "", "", fmt.Errorf("user not found")
	}

	// Mark token as used
	now := time.Now()
	magicLink.Used = true
	magicLink.UsedAt = &now
	if err := c.storage.SaveMagicLinkToken(ctx, magicLink); err != nil {
		c.logger.Errorf("Failed to mark magic link as used: %v", err)
		// Continue anyway - the important part is returning the user
	}

	// Update user's last login time
	user.LastLoginAt = &now
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("Failed to update user last login: %v", err)
		// Continue anyway
	}

	c.logger.Infof("Magic link verified successfully for user %s (email: %s)", user.ID, user.Email)
	return user, magicLink.CallbackURL, magicLink.State, nil
}

// SendMagicLinkEmail sends a magic link email to the user.
func (c *Connector) SendMagicLinkEmail(ctx context.Context, email, magicLinkURL string) error {
	// Check if email is configured
	if c.config.Email.SMTP.Host == "" {
		return fmt.Errorf("email is not configured")
	}

	// Build email content
	subject := "Your Enopax login link"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .container {
            background: #f9f9f9;
            border-radius: 8px;
            padding: 30px;
            margin: 20px 0;
        }
        .button {
            display: inline-block;
            padding: 12px 24px;
            background: #007bff;
            color: #ffffff;
            text-decoration: none;
            border-radius: 4px;
            margin: 20px 0;
        }
        .button:hover {
            background: #0056b3;
        }
        .footer {
            font-size: 12px;
            color: #666;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #ddd;
        }
        .warning {
            background: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 12px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Your Enopax login link</h1>
        <p>Click the button below to log in to your Enopax account:</p>
        <a href="%s" class="button">Log in to Enopax</a>
        <p>Or copy and paste this link into your browser:</p>
        <p style="word-break: break-all; background: #fff; padding: 10px; border-radius: 4px; border: 1px solid #ddd;">%s</p>
        <div class="warning">
            <strong>⚠️ This link expires in %d minutes</strong>
        </div>
        <p>If you didn't request this login link, you can safely ignore this email.</p>
    </div>
    <div class="footer">
        <p>This is an automated email from Enopax Authentication. Please do not reply to this email.</p>
        <p>If you have any questions, please contact our support team.</p>
    </div>
</body>
</html>
`, subject, magicLinkURL, magicLinkURL, c.config.MagicLink.TTL/60)

	// Send email using the email sender interface
	if c.emailSender != nil {
		return c.emailSender.SendEmail(email, subject, body)
	}

	// If no email sender is configured, return an error
	return fmt.Errorf("email sender not configured")
}

// EmailSender is an interface for sending emails.
type EmailSender interface {
	SendEmail(to, subject, body string) error
}

// SetEmailSender sets the email sender implementation.
func (c *Connector) SetEmailSender(sender EmailSender) {
	c.emailSender = sender
}

// MagicLinkClaims represents JWT claims for magic links (alternative to random tokens).
type MagicLinkClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateJWTMagicLink generates a JWT-based magic link token (alternative implementation).
// This is not currently used but provided as an alternative to random tokens.
func (c *Connector) GenerateJWTMagicLink(userID, email string, secret []byte) (string, error) {
	now := time.Now()
	ttl := time.Duration(c.config.MagicLink.TTL) * time.Second

	claims := MagicLinkClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "dex-local-enhanced",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ValidateJWTMagicLink validates a JWT-based magic link token (alternative implementation).
// This is not currently used but provided as an alternative to random tokens.
func (c *Connector) ValidateJWTMagicLink(tokenString string, secret []byte) (*MagicLinkClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MagicLinkClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*MagicLinkClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}
