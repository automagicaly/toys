package rate_limiter

import (
	"fmt"
	"log"

	sl "lorde.tech/toys/skiplist"
)

type RateLimiter struct {
	rate   int64
	ipMap  *sl.SkipList[*_bucket]
	logger *log.Logger
}

func (rl *RateLimiter) fetchByIp(ip string) (*_bucket, error) {
	found, bucket := rl.ipMap.Search(ip)
	if !found {
		bucket = newBucket(rl.rate)
		if err := rl.ipMap.Insert(ip, bucket); err != nil {
			return nil, err
		}
	}
	return bucket, nil
}

func NewRateLimiter(rate int64, logger *log.Logger) *RateLimiter {
	return &RateLimiter{
		rate:   rate,
		ipMap:  sl.NewSkiplist[*_bucket](),
		logger: logger,
	}
}

func (rl *RateLimiter) ShouldServe(ip string) bool {
	bucket, err := rl.fetchByIp(ip)
	if err != nil {
		return false
	}
	return bucket.useToken()
}

func (rl *RateLimiter) Compact() {
	for k, v := range rl.ipMap.Iter() {
		if v.isOld() {
			rl.ipMap.Remove(k)
		}
	}
}

func (rl *RateLimiter) wrap(f http.HandlerFunc) http.HandlerFunc {
	red := func(s string) string {
		return fmt.Sprintf("\x1b[1;31m%s\x1b[0m", s)
	}
	erro_log_format := red("REQUEST BLOCKED") + " -> %s"
	return func(w http.ResponseWriter, r *http.Request) {
		log := newLogger(r, "RATE LIMIT")
		if !rl.ShouldServe(getClientIp(r)) {
			log.Error(erro_log_format, r.URL.Path)
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		f(w, r)
	}
}
