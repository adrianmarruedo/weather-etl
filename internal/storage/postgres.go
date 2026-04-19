package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/adrianmarruedo/weather-etl/internal/model"
	_ "github.com/lib/pq"
)

type DB struct {
	conn   *sql.DB
	dbName string
}

func New(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}
	dbName := ""
	if u, err := url.Parse(dsn); err == nil {
		dbName = u.Path[1:]
	}
	return &DB{conn: conn, dbName: dbName}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping() error {
	return db.conn.Ping()
}

func (db *DB) Table(name string) string {
	return db.dbName + "." + name
}

func (db *DB) InsertRaw(raw *model.RawWeather) error {
	rawJSON, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to marshal raw weather: %w", err)
	}

	_, err = db.conn.Exec(
		`INSERT INTO weather_raw (ingested_at, city, country, raw_data) VALUES ($1, $2, $3, $4)`,
		raw.IngestedAt, raw.Name, raw.Sys.Country, rawJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert raw record: %w", err)
	}
	return nil
}

// InsertProcessed upserts a processed record, deduplicating on (city, observed_at).
// Returns (true, nil) when a new row was inserted, (false, nil) when skipped as duplicate.
func (db *DB) InsertProcessed(p *model.ProcessedWeather) (inserted bool, err error) {
	var id int
	err = db.conn.QueryRow(
		`INSERT INTO weather_processed
			(observed_at, ingested_at, city, country, temperature_celsius, feels_like_celsius,
			 humidity_percent, pressure_hpa, weather_condition, weather_description,
			 wind_speed_ms, wind_direction_deg, visibility_m, cloudiness_percent)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (city, observed_at) DO NOTHING
		 RETURNING id`,
		p.ObservedAt, p.IngestedAt, p.City, p.Country, p.TemperatureCelsius, p.FeelsLikeCelsius,
		p.HumidityPercent, p.PressureHPa, p.WeatherCondition, p.WeatherDescription,
		p.WindSpeedMS, p.WindDirectionDeg, p.VisibilityM, p.CloudinessPercent,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to upsert processed record: %w", err)
	}
	return true, nil
}
