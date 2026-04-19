package model

import "time"

type RawWeather struct {
	IngestedAt time.Time `json:"ingested_at"`
	Coord      struct {
		Lon float64 `json:"lon"`
		Lat float64 `json:"lat"`
	} `json:"coord"`
	Weather []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Pressure  int     `json:"pressure"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Visibility int `json:"visibility"`
	Wind       struct {
		Speed float64 `json:"speed"`
		Deg   int     `json:"deg"`
	} `json:"wind"`
	Clouds struct {
		All int `json:"all"`
	} `json:"clouds"`
	Dt  int64 `json:"dt"`
	Sys struct {
		Country string `json:"country"`
		Sunrise int64  `json:"sunrise"`
		Sunset  int64  `json:"sunset"`
	} `json:"sys"`
	Name string `json:"name"`
}

type ProcessedWeather struct {
	ObservedAt         time.Time `json:"observed_at"`
	IngestedAt         time.Time `json:"ingested_at"`
	City               string    `json:"city"`
	Country            string    `json:"country"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
	FeelsLikeCelsius   float64   `json:"feels_like_celsius"`
	HumidityPercent    int       `json:"humidity_percent"`
	PressureHPa        int       `json:"pressure_hpa"`
	WeatherCondition   string    `json:"weather_condition"`
	WeatherDescription string    `json:"weather_description"`
	WindSpeedMS        float64   `json:"wind_speed_ms"`
	WindDirectionDeg   int       `json:"wind_direction_deg"`
	VisibilityM        int       `json:"visibility_m"`
	CloudinessPercent  int       `json:"cloudiness_percent"`
}
