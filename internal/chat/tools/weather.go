package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/openai/openai-go/v2"
)

type WeatherArgs struct {
	Location     string `json:"location"`
	ForecastDays int    `json:"forecastDays"`
}

type WeatherTool struct{}

func (w WeatherTool) Name() string        { return "get_weather" }
func (w WeatherTool) Description() string { return "Get weather at the given location" }
func (w WeatherTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]string{
				"type":        "string",
				"description": "The location to get the weather for",
			},
			"forecastDays": map[string]string{
				"type":        "integer",
				"description": "The number of days to include in the weather forecast",
			},
		},
		"required": []string{"location"},
	}
}

func (w WeatherTool) Handle(ctx context.Context, args json.RawMessage) (string, error) {
	var wa WeatherArgs
	if err := json.Unmarshal(args, &wa); err != nil {
		return "failed to parse weather arguments", err
	}
	weatherAPIKey := os.Getenv("WEATHER_API_KEY")
	url := "https://api.weatherapi.com/v1/"
	params := fmt.Sprintf("?key=%s&q=%s", weatherAPIKey, wa.Location)
	var apiPath string
	if wa.ForecastDays > 0 {
		apiPath = "forecast.json"
		params += fmt.Sprintf("&days=%d", wa.ForecastDays)
	} else {
		apiPath = "current.json"
	}
	url += apiPath + params
	resp, err := http.Get(url)
	if err != nil {
		return "failed to get weather data", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "failed to read weather data", err
	}
	return string(body), nil
}

func init() {
	Register(WeatherTool{})
}
