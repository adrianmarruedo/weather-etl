CREATE TABLE IF NOT EXISTS weather_raw (
    id          SERIAL PRIMARY KEY,
    ingested_at TIMESTAMPTZ NOT NULL,
    city        VARCHAR(100),
    country     VARCHAR(10),
    raw_data    JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_weather_raw_ingested_at ON weather_raw (ingested_at DESC);
CREATE INDEX IF NOT EXISTS idx_weather_raw_city ON weather_raw (city);

CREATE TABLE IF NOT EXISTS weather_processed (
    id                  SERIAL PRIMARY KEY,
    observed_at         TIMESTAMPTZ NOT NULL,
    ingested_at         TIMESTAMPTZ NOT NULL,
    city                VARCHAR(100),
    country             VARCHAR(10),
    temperature_celsius NUMERIC(5,2),
    feels_like_celsius  NUMERIC(5,2),
    humidity_percent    INT,
    pressure_hpa        INT,
    weather_condition   VARCHAR(50),
    weather_description VARCHAR(200),
    wind_speed_ms       NUMERIC(5,2),
    wind_direction_deg  INT,
    visibility_m        INT,
    cloudiness_percent  INT
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_weather_processed_city_observed_at ON weather_processed (city, observed_at);
CREATE INDEX IF NOT EXISTS idx_weather_processed_observed_at ON weather_processed (observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_weather_processed_city ON weather_processed (city);
