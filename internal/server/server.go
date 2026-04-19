package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	APIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "etl_api_requests_total",
		Help: "Total number of OpenWeatherMap API requests.",
	}, []string{"status"})

	APILatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "etl_api_latency_seconds",
		Help:    "OpenWeatherMap API response latency in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	TransformErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etl_transform_errors_total",
		Help: "Total number of transformation errors.",
	})

	RecordsSavedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "etl_records_saved_total",
		Help: "Total number of records persisted.",
	}, []string{"type", "backend"})

	LastSuccessfulRunTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "etl_last_successful_run_timestamp",
		Help: "Unix timestamp of the last fully successful ETL cycle.",
	})

	RecordsSkippedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "etl_records_skipped_total",
		Help: "Total records skipped as duplicates (upsert DO NOTHING).",
	}, []string{"type", "backend"})
)

// Pinger is satisfied by *storage.DB — kept as an interface so server
// doesn't import the storage package.
type Pinger interface {
	Ping() error
}

const staleCycleThreshold = 2 * time.Minute

type Server struct {
	startTime   time.Time
	port        string
	db          Pinger
	lastSuccess atomic.Int64 // unix nanoseconds, 0 = never ran
}

func New(port string, db Pinger) *Server {
	s := &Server{startTime: time.Now(), port: port, db: db}
	return s
}

// RecordSuccess is called by the ETL loop after each fully successful cycle.
func (s *Server) RecordSuccess() {
	s.lastSuccess.Store(time.Now().UnixNano())
	LastSuccessfulRunTimestamp.SetToCurrentTime()
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(fmt.Sprintf(":%s", s.port), mux)
}

func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	dbStatus := "ok"
	if err := s.db.Ping(); err != nil {
		dbStatus = fmt.Sprintf("error: %s", err.Error())
	}

	etlStatus := "ok"
	var lastCycleSecondsAgo float64
	ns := s.lastSuccess.Load()
	if ns == 0 {
		etlStatus = "never ran"
	} else {
		lastCycleSecondsAgo = time.Since(time.Unix(0, ns)).Seconds()
		if lastCycleSecondsAgo > staleCycleThreshold.Seconds() {
			etlStatus = "stale"
		}
	}

	degraded := dbStatus != "ok" || (etlStatus != "ok" && etlStatus != "never ran")
	overallStatus := "ok"
	if degraded {
		overallStatus = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if degraded {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status":                  overallStatus,
		"uptime_seconds":          int(time.Since(s.startTime).Seconds()),
		"last_etl_cycle_seconds_ago": lastCycleSecondsAgo,
		"checks": map[string]string{
			"postgres": dbStatus,
			"etl":      etlStatus,
		},
	})
}
