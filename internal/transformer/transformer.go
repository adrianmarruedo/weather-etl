package transformer

import (
	"fmt"
	"time"

	"github.com/adrianmarruedo/weather-etl/internal/model"
)

func Transform(raw *model.RawWeather) (*model.ProcessedWeather, error) {
	if raw == nil {
		return nil, fmt.Errorf("raw weather data is nil")
	}

	condition := ""
	description := ""
	if len(raw.Weather) > 0 {
		condition = raw.Weather[0].Main
		description = raw.Weather[0].Description
	}

	return &model.ProcessedWeather{
		ObservedAt:         time.Unix(raw.Dt, 0).UTC(),
		IngestedAt:         raw.IngestedAt,
		City:               raw.Name,
		Country:            raw.Sys.Country,
		TemperatureCelsius: raw.Main.Temp,
		FeelsLikeCelsius:   raw.Main.FeelsLike,
		HumidityPercent:    raw.Main.Humidity,
		PressureHPa:        raw.Main.Pressure,
		WeatherCondition:   condition,
		WeatherDescription: description,
		WindSpeedMS:        raw.Wind.Speed,
		WindDirectionDeg:   raw.Wind.Deg,
		VisibilityM:        raw.Visibility,
		CloudinessPercent:  raw.Clouds.All,
	}, nil
}
