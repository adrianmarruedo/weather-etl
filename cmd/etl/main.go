package main

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/adrianmarruedo/weather-etl/internal/config"
	"github.com/adrianmarruedo/weather-etl/internal/extractor"
	"github.com/adrianmarruedo/weather-etl/internal/loader"
	"github.com/adrianmarruedo/weather-etl/internal/logger"
	"github.com/adrianmarruedo/weather-etl/internal/server"
	"github.com/adrianmarruedo/weather-etl/internal/storage"
	"github.com/adrianmarruedo/weather-etl/internal/transformer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("logs", 0o755); err != nil {
		slog.Error("failed to create logs dir", "error", err)
		os.Exit(1)
	}

	log, logFile, err := logger.New(cfg.LogLevel, "logs/etl.log")
	if err != nil {
		slog.Error("failed to init logger", "error", err)
		os.Exit(1)
	}
	defer logFile.Close()

	db, err := storage.New(cfg.DBDSN)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("connected to postgres")

	ldr, err := loader.New("data/raw", "data/processed")
	if err != nil {
		log.Error("failed to init loader", "error", err)
		os.Exit(1)
	}

	clients := make([]*extractor.Client, len(cfg.OWMCities))
	for i, city := range cfg.OWMCities {
		clients[i] = extractor.NewClient(cfg.OWMAPIKey, city, cfg.OWMUnits)
	}

	srv := server.New(cfg.HTTPPort, db)
	go func() {
		log.Info("http server starting", "port", cfg.HTTPPort)
		if err := srv.Start(); err != nil {
			log.Error("http server error", "error", err)
		}
	}()

	log.Info("etl pipeline starting", "interval_seconds", cfg.FetchIntervalSeconds, "cities", cfg.OWMCities)

	runAllCities(clients, ldr, db, srv, log)

	ticker := time.NewTicker(time.Duration(cfg.FetchIntervalSeconds) * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runAllCities(clients, ldr, db, srv, log)
		case sig := <-quit:
			log.Info("shutting down", "signal", sig)
			return
		}
	}
}

func runAllCities(clients []*extractor.Client, ldr *loader.Loader, db *storage.DB, srv *server.Server, log *slog.Logger) {
	var wg sync.WaitGroup
	for _, client := range clients {
		wg.Add(1)
		go func(c *extractor.Client) {
			defer wg.Done()
			runCycle(c, ldr, db, srv, log)
		}(client)
	}
	wg.Wait()
}

func runCycle(
	client *extractor.Client,
	ldr *loader.Loader,
	db *storage.DB,
	srv *server.Server,
	log *slog.Logger,
) {
	raw, latency, err := client.Fetch()
	server.APILatencySeconds.Observe(latency.Seconds())
	if err != nil {
		server.APIRequestsTotal.WithLabelValues("failure").Inc()
		log.Error("api request failed", "error", err, "latency_ms", latency.Milliseconds())
		return
	}
	server.APIRequestsTotal.WithLabelValues("success").Inc()
	log.Info("api request succeeded", "city", raw.Name, "latency_ms", latency.Milliseconds())

	if path, err := ldr.SaveRaw(raw); err != nil {
		log.Error("failed to save raw file", "error", err, "city", raw.Name)
	} else {
		server.RecordsSavedTotal.WithLabelValues("raw", "file").Inc()
		log.Info("raw data saved to file", "file", path, "city", raw.Name)
	}

	if err := db.InsertRaw(raw); err != nil {
		log.Error("failed to insert raw record to postgres", "error", err, "city", raw.Name)
	} else {
		server.RecordsSavedTotal.WithLabelValues("raw", "postgres").Inc()
		log.Info("raw data saved to postgres", "table", db.Table("weather_raw"), "city", raw.Name)
	}

	processed, err := transformer.Transform(raw)
	if err != nil {
		server.TransformErrorsTotal.Inc()
		log.Error("transformation failed", "error", err, "city", raw.Name)
		return
	}
	log.Info("transformation succeeded", "city", processed.City, "temp_c", processed.TemperatureCelsius)

	if path, err := ldr.SaveProcessed(processed); err != nil {
		log.Error("failed to save processed file", "error", err, "city", processed.City)
	} else {
		server.RecordsSavedTotal.WithLabelValues("processed", "file").Inc()
		log.Info("processed data saved to file", "file", path, "city", processed.City)
	}

	if inserted, err := db.InsertProcessed(processed); err != nil {
		log.Error("failed to upsert processed record to postgres", "error", err, "city", processed.City)
	} else if inserted {
		server.RecordsSavedTotal.WithLabelValues("processed", "postgres").Inc()
		log.Info("processed data saved to postgres", "table", db.Table("weather_processed"), "city", processed.City)
	} else {
		server.RecordsSkippedTotal.WithLabelValues("processed", "postgres").Inc()
		log.Info("processed record skipped as duplicate", "city", processed.City, "observed_at", processed.ObservedAt)
	}

	srv.RecordSuccess()
	log.Info("etl cycle completed successfully", "city", processed.City)
}
