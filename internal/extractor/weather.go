package extractor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/adrianmarruedo/weather-etl/internal/model"
)

const owmBaseURL = "https://api.openweathermap.org/data/2.5/weather"

type Client struct {
	apiKey  string
	city    string
	units   string
	httpClient *http.Client
}

func NewClient(apiKey, city, units string) *Client {
	return &Client{
		apiKey: apiKey,
		city:   city,
		units:  units,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Fetch() (*model.RawWeather, time.Duration, error) {
	url := fmt.Sprintf("%s?q=%s&units=%s&appid=%s", owmBaseURL, c.city, c.units, c.apiKey)

	start := time.Now()
	resp, err := c.httpClient.Get(url)
	latency := time.Since(start)

	if err != nil {
		return nil, latency, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, latency, fmt.Errorf("api returned status %d", resp.StatusCode)
	}

	var raw model.RawWeather
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, latency, fmt.Errorf("failed to decode response: %w", err)
	}

	raw.IngestedAt = time.Now().UTC()
	return &raw, latency, nil
}
