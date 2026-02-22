# Skill: weather

## Description
Get current weather and forecasts for any location using wttr.in. Use when the user asks about weather, temperature, rain, forecasts, or travel weather. No API key needed.

## Instructions
Use the `shell` tool with `curl` to fetch weather from wttr.in.

### Current Weather (one-liner)
```bash
curl -s "wttr.in/London?format=3"
```

### Detailed Current Conditions
```bash
curl -s "wttr.in/London?0"
```

### 3-Day Forecast
```bash
curl -s "wttr.in/London"
```

### Custom Format (best for chat responses)
```bash
curl -s "wttr.in/London?format=%l:+%c+%t+(feels+like+%f),+%w+wind,+%h+humidity"
```

### Rain Check
```bash
curl -s "wttr.in/London?format=%l:+%c+%p+precipitation"
```

### JSON Output (for structured data)
```bash
curl -s "wttr.in/London?format=j1"
```

### Format Codes
- `%c` Weather emoji
- `%t` Temperature
- `%f` Feels like
- `%w` Wind
- `%h` Humidity
- `%p` Precipitation
- `%l` Location

### Tips
- Use `+` for spaces in city names: `New+York`
- Airport codes work: `wttr.in/ORD`
- Don't spam requests (rate limited)
- Default units are metric. Append `?u` for USCS or `?m` for metric explicitly.

## Tools
- name: weather_current
  description: Get current weather for a location
  command: curl -s "wttr.in/{{location}}?format=%l:+%c+%t+(feels+like+%f),+%w+wind,+%h+humidity"
- name: weather_forecast
  description: Get 3-day forecast for a location
  command: curl -s "wttr.in/{{location}}"
- name: weather_rain
  description: Check precipitation for a location
  command: curl -s "wttr.in/{{location}}?format=%l:+%c+%p+precipitation"
- name: weather_json
  description: Get weather data as JSON
  command: curl -s "wttr.in/{{location}}?format=j1"
