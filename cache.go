package main

import (
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	bucketCleanup = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "cache_cleanup_seconds",
			Help: "Time it takes to loop throught the cached buckets",
		},
		[]string{"kind"},
	)
	cacheInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_instances",
			Help: "Number of cache instances running",
		},
		[]string{"kind"},
	)
	bucketSlots = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_buckets",
			Help: "Number of buckets in cache",
		},
		[]string{"kind"},
	)
	bucketTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_requests_total",
			Help: "Number of cache requests",
		},
		[]string{"kind"},
	)
	bucketMiss = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_miss_total",
			Help: "Number of cache misses",
		},
		[]string{"kind"},
	)
)

func init() {
	prometheus.MustRegister(bucketCleanup)
	prometheus.MustRegister(cacheInstances)
	prometheus.MustRegister(bucketSlots)
	prometheus.MustRegister(bucketTotal)
	prometheus.MustRegister(bucketMiss)
}

// Sizer is a function that returns the bucket size for the IP Address provided.
type Sizer func(net.IP) (fill, max int)

// bucket implements a token bucket.
// Each tick, the bucket is filled with 'Fill' tokens.
// Tokens are capped at 'Max'.
type bucket struct {
	fill   int
	max    int
	count  int
	unused int
}

// Cache holds a cache of token buckets
type Cache struct {
	buckets map[uint64]bucket
	sync.Mutex
	ticker   *time.Ticker
	sizer    Sizer
	slots    prometheus.Gauge
	requests prometheus.Counter
	misses   prometheus.Counter
}

// NewCache builds a new cache of token buckets.
// The buckets fill at the given rate,
// A bucket not used for 'unused' ticks is removed,
// Bucket sizes are given by the sizer.
func NewCache(kind string, rate time.Duration, unused int, sizer Sizer) *Cache {
	c := &Cache{
		buckets:  make(map[uint64]bucket),
		sizer:    sizer,
		ticker:   time.NewTicker(rate),
		slots:    bucketSlots.WithLabelValues(kind),
		requests: bucketTotal.WithLabelValues(kind),
		misses:   bucketMiss.WithLabelValues(kind),
	}
	go func() {
		instances := cacheInstances.WithLabelValues(kind)
		instances.Inc()
		defer instances.Dec()
		cleanup := bucketCleanup.WithLabelValues(kind)
		for range c.ticker.C {
			t := time.Now()
			c.Lock()
			for k, v := range c.buckets {
				v.count += v.fill
				if v.count > v.max {
					v.count = v.max
				}
				v.unused += 1
				if v.unused > unused {
					c.slots.Dec()
					delete(c.buckets, k)
				} else {
					c.buckets[k] = v
				}
			}
			c.Unlock()
			cleanup.Observe(time.Now().Sub(t).Seconds())
		}
	}()
	return c
}

// Stop the ticker
func (c *Cache) Stop() {
	c.ticker.Stop()
}

// Check if the bucket allows the request
func (c *Cache) Check(ip net.IP, hash uint64, tokens int) bool {
	c.requests.Inc()
	c.Lock()
	defer c.Unlock()
	v, ok := c.buckets[hash]
	if !ok {
		c.slots.Inc()
		c.misses.Inc()
		fill, max := c.sizer(ip)
		v.fill = fill
		v.max = max
		v.count = v.fill
	}
	v.unused = 0
	v.count -= tokens
	if v.count < -v.max {
		v.count = -v.max
	}
	c.buckets[hash] = v
	return v.count >= 0
}
