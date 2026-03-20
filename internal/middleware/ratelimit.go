package middleware

import (
	"sync"
	"time"

	"github.com/AugustLigh/GoMino/internal/response"
	"github.com/gofiber/fiber/v3"
)

// rateLimitEntry tracks request counts within a time window
type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimiter provides in-memory IP-based rate limiting
type RateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	limit    int
	window   time.Duration
	stopChan chan struct{}
}

// NewRateLimiter creates a rate limiter with the given limit per window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries:  make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
		stopChan: make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// cleanup removes expired entries every minute
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, entry := range rl.entries {
				if now.After(entry.windowEnd) {
					delete(rl.entries, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopChan:
			return
		}
	}
}

// Allow checks if a request from the given key is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[key]

	if !exists || now.After(entry.windowEnd) {
		rl.entries[key] = &rateLimitEntry{
			count:     1,
			windowEnd: now.Add(rl.window),
		}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}

// GetCount returns the current count for a key
func (rl *RateLimiter) GetCount(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.entries[key]
	if !exists || time.Now().After(entry.windowEnd) {
		return 0
	}
	return entry.count
}

// Global rate limiters
var (
	// GeneralLimiter: 120 requests per minute per IP
	GeneralLimiter *RateLimiter

	// AuthLimiter: 10 requests per minute per IP for auth endpoints
	AuthLimiter *RateLimiter

	// EmailLimiter: 3 emails per 10 minutes per email address
	EmailLimiter *RateLimiter

	// LoginFailLimiter: 5 failed login attempts per 15 minutes per IP
	LoginFailLimiter *RateLimiter

	// RegisterLimiter: 3 registrations per hour per IP
	RegisterLimiter *RateLimiter

	// MessageLimiter: 30 messages per minute per user
	MessageLimiter *RateLimiter
)

// InitRateLimiters initializes all rate limiters
func InitRateLimiters() {
	GeneralLimiter = NewRateLimiter(120, 1*time.Minute)
	AuthLimiter = NewRateLimiter(10, 1*time.Minute)
	EmailLimiter = NewRateLimiter(3, 10*time.Minute)
	LoginFailLimiter = NewRateLimiter(5, 15*time.Minute)
	RegisterLimiter = NewRateLimiter(3, 1*time.Hour)
	MessageLimiter = NewRateLimiter(30, 1*time.Minute)
}

// RateLimitMiddleware applies general rate limiting per IP
func RateLimitMiddleware(c fiber.Ctx) error {
	if GeneralLimiter == nil {
		return c.Next()
	}

	ip := c.IP()
	if !GeneralLimiter.Allow(ip) {
		return c.Status(fiber.StatusTooManyRequests).JSON(response.NewError(response.StatusTooManyRequests))
	}
	return c.Next()
}

// AuthRateLimitMiddleware applies stricter rate limiting for auth endpoints
func AuthRateLimitMiddleware(c fiber.Ctx) error {
	if AuthLimiter == nil {
		return c.Next()
	}

	ip := c.IP()
	if !AuthLimiter.Allow(ip) {
		return c.Status(fiber.StatusTooManyRequests).JSON(response.NewError(response.StatusTooManyRequests))
	}
	return c.Next()
}

// CheckEmailRateLimit checks if sending email to this address is allowed
func CheckEmailRateLimit(email string) bool {
	if EmailLimiter == nil {
		return true
	}
	return EmailLimiter.Allow(email)
}

// RecordLoginFailure records a failed login attempt for the IP
func RecordLoginFailure(ip string) {
	if LoginFailLimiter == nil {
		return
	}
	LoginFailLimiter.Allow(ip) // increments the counter
}

// IsLoginBlocked checks if the IP is blocked due to too many failed attempts
func IsLoginBlocked(ip string) bool {
	if LoginFailLimiter == nil {
		return false
	}
	return LoginFailLimiter.GetCount(ip) >= LoginFailLimiter.limit
}

// CheckRegisterRateLimit checks if registration from this IP is allowed
func CheckRegisterRateLimit(ip string) bool {
	if RegisterLimiter == nil {
		return true
	}
	return RegisterLimiter.Allow(ip)
}

// CheckMessageRateLimit checks if a user can send more messages
func CheckMessageRateLimit(userID string) bool {
	if MessageLimiter == nil {
		return true
	}
	return MessageLimiter.Allow(userID)
}
