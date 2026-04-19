package transformer

import (
	"testing"
	"time"

	"github.com/adrianmarruedo/weather-etl/internal/model"
)

func baseRaw() *model.RawWeather {
	raw := &model.RawWeather{
		IngestedAt: time.Date(2024, 1, 15, 10, 30, 1, 0, time.UTC),
		Dt:         1705312200,
		Name:       "London",
	}
	raw.Sys.Country = "GB"
	raw.Main.Temp = 8.5
	raw.Main.FeelsLike = 6.1
	raw.Main.Humidity = 82
	raw.Main.Pressure = 1012
	raw.Wind.Speed = 4.6
	raw.Wind.Deg = 250
	raw.Visibility = 10000
	raw.Clouds.All = 100
	raw.Weather = []struct {
		Main        string `json:"main"`
		Description string `json:"description"`
	}{
		{Main: "Clouds", Description: "overcast clouds"},
	}
	return raw
}

func TestTransform_NilInput(t *testing.T) {
	_, err := Transform(nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
}

func TestTransform_FieldMapping(t *testing.T) {
	raw := baseRaw()
	got, err := Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.City != "London" {
		t.Errorf("City: want London, got %s", got.City)
	}
	if got.Country != "GB" {
		t.Errorf("Country: want GB, got %s", got.Country)
	}
	if got.TemperatureCelsius != 8.5 {
		t.Errorf("TemperatureCelsius: want 8.5, got %f", got.TemperatureCelsius)
	}
	if got.WeatherCondition != "Clouds" {
		t.Errorf("WeatherCondition: want Clouds, got %s", got.WeatherCondition)
	}
	if got.WeatherDescription != "overcast clouds" {
		t.Errorf("WeatherDescription: want overcast clouds, got %s", got.WeatherDescription)
	}
}

func TestTransform_TimestampsAreUTC(t *testing.T) {
	raw := baseRaw()
	got, err := Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ObservedAt.Location() != time.UTC {
		t.Errorf("ObservedAt timezone: want UTC, got %s", got.ObservedAt.Location())
	}
	if got.IngestedAt.Location() != time.UTC {
		t.Errorf("IngestedAt timezone: want UTC, got %s", got.IngestedAt.Location())
	}

	expectedObserved := time.Unix(1705312200, 0).UTC()
	if !got.ObservedAt.Equal(expectedObserved) {
		t.Errorf("ObservedAt: want %v, got %v", expectedObserved, got.ObservedAt)
	}
	if !got.IngestedAt.Equal(raw.IngestedAt) {
		t.Errorf("IngestedAt: want %v, got %v", raw.IngestedAt, got.IngestedAt)
	}
}

func TestTransform_EmptyWeatherArray(t *testing.T) {
	raw := baseRaw()
	raw.Weather = nil

	got, err := Transform(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WeatherCondition != "" {
		t.Errorf("WeatherCondition: want empty string, got %s", got.WeatherCondition)
	}
	if got.WeatherDescription != "" {
		t.Errorf("WeatherDescription: want empty string, got %s", got.WeatherDescription)
	}
}
