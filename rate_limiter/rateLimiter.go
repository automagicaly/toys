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
	rate        int64
	bucketMap   *sl.SkipList[*_bucket]
	logger      *slog.Logger
	tracer      trace.Tracer
	meter       metric.Meter
	blocksCnt   metric.Int64Counter
	requestsCnt metric.Int64Counter
}

func NewRateLimiter(rate int64) (*RateLimiter, error) {
	meter := otel.Meter(name)
	blocksCnt, err := meter.Int64Counter("requests.blocked", metric.WithDescription("Number of blocked requests"))
	if err != nil {
		return nil, err
	}
	requestsCnt, err := meter.Int64Counter("requests.count", metric.WithDescription("Total number of requests"))
	if err != nil {
		return nil, err
	}

	return &RateLimiter{
		rate:        rate,
		bucketMap:   sl.NewSkiplist[*_bucket](),
		logger:      otelslog.NewLogger(name),
		tracer:      otel.Tracer(name),
		meter:       meter,
		blocksCnt:   blocksCnt,
		requestsCnt: requestsCnt,
	}, nil
}

func (rl *RateLimiter) ShouldServe(id string) bool {
	bucket, err := rl.fetchBucket(id)
	if err != nil {
		return false
	}
	return bucket.useToken()
}

func (rl *RateLimiter) LimitByIP(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := rl.tracer.Start(r.Context(), "rate limited request by IP")
		defer span.End()
		ip := commons.GetClientIp(r)
		ipAttribute := attribute.String("ip", ip)
		pathAttribute := attribute.String("path", r.URL.Path)
		span.SetAttributes(ipAttribute, pathAttribute)
		rl.requestsCnt.Add(ctx, 1, metric.WithAttributes(ipAttribute, pathAttribute))

		if !rl.ShouldServe(ip) {
			span.SetAttributes(attribute.Bool("blocked", true))
			rl.blocksCnt.Add(ctx, 1, metric.WithAttributes(
				ipAttribute,
				pathAttribute,
			))
			rl.logger.InfoContext(ctx, "Request Blocked", "ip", ip, "path", r.URL.Path)
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		span.SetAttributes(attribute.Bool("blocked", false))
		f(w, r)
	}
}

func (rl *RateLimiter) LimitByCustomID(f http.HandlerFunc, getID func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := rl.tracer.Start(r.Context(), "rate limited request by IP")
		defer span.End()
		id := getID(r)
		idAttribute := attribute.String("id", id)
		pathAttribute := attribute.String("path", r.URL.Path)
		span.SetAttributes(idAttribute, pathAttribute)
		rl.requestsCnt.Add(ctx, 1, metric.WithAttributes(idAttribute, pathAttribute))

		if !rl.ShouldServe(id) {
			span.SetAttributes(attribute.Bool("blocked", true))
			rl.blocksCnt.Add(ctx, 1, metric.WithAttributes(
				idAttribute,
				pathAttribute,
			))
			rl.logger.InfoContext(ctx, "Request Blocked", "id", id, "path", r.URL.Path)
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		span.SetAttributes(attribute.Bool("blocked", false))
		f(w, r)

	}
}

func (rl *RateLimiter) Compact() {
	for k, v := range rl.bucketMap.Iter() {
		if v.isOld() {
			rl.bucketMap.Remove(k)
		}
	}
}

func (rl *RateLimiter) fetchBucket(id string) (*_bucket, error) {
	found, bucket := rl.bucketMap.Search(id)
	if !found {
		bucket = newBucket(rl.rate)
		if err := rl.bucketMap.Insert(id, bucket); err != nil {
			return nil, err
		}
	}
	return bucket, nil
}
