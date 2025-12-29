package rate_limiter

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"lorde.tech/toys/commons"
	sl "lorde.tech/toys/skiplist"
)

const name = "lorde.tech/toys/rate_limiter"

type RateLimiter struct {
	rate      int64
	ipMap     *sl.SkipList[*_bucket]
	logger    *slog.Logger
	tracer    trace.Tracer
	meter     metric.Meter
	blocksCnt metric.Int64Counter
}

func NewRateLimiter(rate int64) *RateLimiter {
	meter := otel.Meter(name)
	blocksCnt, err := meter.Int64Counter("requests.blocked", metric.WithDescription("Number of blocked requests"))

	commons.DieOnError(err)

	return &RateLimiter{
		rate:      rate,
		ipMap:     sl.NewSkiplist[*_bucket](),
		logger:    otelslog.NewLogger(name),
		tracer:    otel.Tracer(name),
		meter:     meter,
		blocksCnt: blocksCnt,
	}
}

func (rl *RateLimiter) LimitByIP(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := rl.tracer.Start(r.Context(), "rate limited request")
		defer span.End()
		ip := commons.GetClientIp(r)
		ipAttribute := attribute.String("ip", ip)
		pathAttribute := attribute.String("path", r.URL.Path)
		span.SetAttributes(ipAttribute, pathAttribute)

		if !rl.shouldServe(ip) {
			rl.blocksCnt.Add(ctx, 1, metric.WithAttributes(
				ipAttribute,
				pathAttribute,
			))
			rl.logger.InfoContext(ctx, "Request Blocked", "ip", ip, "path", r.URL.Path)
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		f(w, r)
	}
}

func (rl *RateLimiter) Compact() {
	for k, v := range rl.ipMap.Iter() {
		if v.isOld() {
			rl.ipMap.Remove(k)
		}
	}
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

func (rl *RateLimiter) shouldServe(ip string) bool {
	bucket, err := rl.fetchByIp(ip)
	if err != nil {
		return false
	}
	return bucket.useToken()
}
