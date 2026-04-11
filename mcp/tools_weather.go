package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/weather"
)

// getWeatherTool returns the MCP tool definition for weather forecasts.
func getWeatherTool() ToolDef {
	return ToolDef{
		Name:        "get_weather",
		Title:       "Weather Forecast",
		Description: "Get a weather forecast for any city using Open-Meteo (free, no API key). Returns up to 14 days of daily forecasts with temperature, precipitation, and conditions.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"city":      {Type: "string", Description: "City name (e.g. Prague, Helsinki, Tokyo)"},
				"from_date": {Type: "string", Description: "Start date (YYYY-MM-DD, default: today)"},
				"to_date":   {Type: "string", Description: "End date (YYYY-MM-DD, default: today+6)"},
			},
			Required: []string{"city"},
		},
		OutputSchema: weatherOutputSchema(),
		Annotations: &ToolAnnotations{
			Title:          "Weather Forecast",
			ReadOnlyHint:   true,
			OpenWorldHint:  true,
			IdempotentHint: true,
		},
	}
}

func weatherOutputSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{"type": "boolean"},
			"city":    map[string]interface{}{"type": "string"},
			"forecasts": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city":          map[string]interface{}{"type": "string"},
						"date":          map[string]interface{}{"type": "string"},
						"temp_max":      map[string]interface{}{"type": "number"},
						"temp_min":      map[string]interface{}{"type": "number"},
						"precipitation": map[string]interface{}{"type": "number"},
						"description":   map[string]interface{}{"type": "string"},
					},
				},
			},
			"error": map[string]interface{}{"type": "string"},
		},
		"required": []string{"success", "city", "forecasts"},
	}
}

func handleGetWeather(ctx context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, progress ProgressFunc) ([]ContentBlock, interface{}, error) {
	city := argString(args, "city")
	fromDate := argString(args, "from_date")
	toDate := argString(args, "to_date")

	if fromDate == "" {
		fromDate = time.Now().Format("2006-01-02")
	}
	if toDate == "" {
		toDate = time.Now().AddDate(0, 0, 6).Format("2006-01-02")
	}

	sendProgress(progress, 10, 100, fmt.Sprintf("Geocoding %s...", city))

	sendProgress(progress, 40, 100, "Fetching forecast from Open-Meteo...")

	result, err := weather.GetForecast(ctx, city, fromDate, toDate)
	if err != nil {
		return nil, nil, err
	}

	sendProgress(progress, 100, 100, fmt.Sprintf("Got %d day forecast", len(result.Forecasts)))

	summary := buildWeatherSummary(result)
	content := []ContentBlock{
		{Type: "text", Text: summary, Annotations: &ContentAnnotation{Audience: []string{"user"}, Priority: 1.0}},
		{Type: "text", Text: "Structured forecast data attached.", Annotations: &ContentAnnotation{Audience: []string{"assistant"}, Priority: 0.5}},
	}
	return content, result, nil
}

func buildWeatherSummary(result *weather.WeatherResult) string {
	if !result.Success {
		return fmt.Sprintf("Weather forecast for %s unavailable: %s", result.City, result.Error)
	}
	if len(result.Forecasts) == 0 {
		return fmt.Sprintf("No forecast data for %s.", result.City)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Weather forecast for %s:\n\n", result.City))
	for _, f := range result.Forecasts {
		emoji := weather.WeatherEmoji(f.Description)
		sb.WriteString(fmt.Sprintf("  %s %s (%s)  %d°/%d°C",
			weather.FormatDateShort(f.Date),
			weather.DayOfWeek(f.Date),
			emoji,
			int(f.TempMin), int(f.TempMax),
		))
		if f.Precipitation > 0 {
			sb.WriteString(fmt.Sprintf("  %.0fmm rain", f.Precipitation))
		}
		sb.WriteString(fmt.Sprintf("  %s\n", f.Description))
	}
	return sb.String()
}
