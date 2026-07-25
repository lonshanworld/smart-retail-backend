package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type rateLimitBucket struct {
	started time.Time
	count   int
}

// RateLimit provides a small process-local guard for authentication and paid
// provider endpoints. Deployments with multiple instances should enforce an
// equivalent limit at the gateway or with a shared store as well.
func RateLimit(max int, window time.Duration) fiber.Handler {
	var mu sync.Mutex
	buckets := make(map[string]rateLimitBucket)

	return func(c *fiber.Ctx) error {
		key := c.IP()
		now := time.Now()
		mu.Lock()
		for bucketKey, candidate := range buckets {
			if !candidate.started.IsZero() && now.Sub(candidate.started) >= window {
				delete(buckets, bucketKey)
			}
		}
		bucket := buckets[key]
		if bucket.started.IsZero() || now.Sub(bucket.started) >= window {
			bucket = rateLimitBucket{started: now}
		}
		bucket.count++
		buckets[key] = bucket
		allowed := bucket.count <= max
		mu.Unlock()
		if !allowed {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"status": "error", "message": "Too many requests"})
		}
		return c.Next()
	}
}
