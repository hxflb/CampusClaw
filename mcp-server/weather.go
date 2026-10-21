package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	geoEndpoint      = "https://geocoding-api.open-meteo.com/v1/search"
	forecastEndpoint = "https://api.open-meteo.com/v1/forecast"
	weatherHTTP      = &http.Client{Timeout: 3 * time.Second}
)

func weatherQuery(question string, args map[string]any) string {
	city := ""
	if args != nil {
		city, _ = args["city"].(string)
	}
	city = strings.TrimSpace(city)
	if city == "" {
		city = cityFromText(question)
	}
	if city == "" {
		return "没有识别出城市。请写明城市，例如北京今天天气怎么样。"
	}
	text, err := lookupWeather(city)
	if err != nil {
		return "暂时查不到" + city + "的天气。"
	}
	return text
}

func cityFromText(question string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		return ""
	}
	if !strings.Contains(q, "天气") && !strings.Contains(strings.ToLower(q), "weather") {
		return ""
	}
	lower := strings.ToLower(q)
	for _, word := range []string{"weather", "today", "now"} {
		if i := strings.Index(lower, word); i >= 0 {
			q = strings.TrimSpace(q[:i] + " " + q[i+len(word):])
			lower = strings.ToLower(q)
		}
	}
	q = strings.TrimSpace(strings.TrimPrefix(q, "in"))
	for _, word := range []string{"请问", "帮我查一下", "帮我查", "查一下", "查询", "看看", "今天", "现在", "明天", "怎么样", "如何", "怎样", "的天气", "天气"} {
		q = strings.ReplaceAll(q, word, "")
	}
	q = strings.Trim(q, "的市呢啊呀？?：:，,。 \t")
	return cleanPlace(q)
}

func cleanPlace(s string) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) < 2 || len(rs) > 20 {
		return ""
	}
	for _, r := range rs {
		if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || r == ' ' || r == '-' {
			continue
		}
		return ""
	}
	return s
}

func lookupWeather(city string) (string, error) {
	geo, err := getJSON(geoEndpoint + "?name=" + url.QueryEscape(city) + "&count=1&language=zh&format=json")
	if err != nil {
		return "", err
	}
	results, _ := geo["results"].([]any)
	if len(results) == 0 {
		return "", fmt.Errorf("no city")
	}
	place, _ := results[0].(map[string]any)
	lat, latOK := place["latitude"].(float64)
	lon, lonOK := place["longitude"].(float64)
	if !latOK || !lonOK {
		return "", fmt.Errorf("no coordinates")
	}
	name, _ := place["name"].(string)
	if strings.TrimSpace(name) == "" {
		name = city
	}
	forecast, err := getJSON(forecastEndpoint + "?latitude=" + trimFloat(lat) + "&longitude=" + trimFloat(lon) + "&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m&timezone=auto")
	if err != nil {
		return "", err
	}
	current, _ := forecast["current"].(map[string]any)
	if current == nil {
		return "", fmt.Errorf("no current")
	}
	temp, ok := current["temperature_2m"].(float64)
	if !ok {
		return "", fmt.Errorf("no temperature")
	}
	code, _ := current["weather_code"].(float64)
	text := name + "当前天气：" + weatherText(int(code)) + "，气温 " + trimFloat(temp) + "℃"
	if humidity, ok := current["relative_humidity_2m"].(float64); ok {
		text += "，湿度 " + trimFloat(humidity) + "%"
	}
	if wind, ok := current["wind_speed_10m"].(float64); ok {
		text += "，风速 " + trimFloat(wind) + " 公里/小时"
	}
	return text + "。", nil
}

func weatherText(code int) string {
	switch code {
	case 0:
		return "晴"
	case 1:
		return "大部晴朗"
	case 2:
		return "多云"
	case 3:
		return "阴"
	case 45, 48:
		return "雾"
	case 51, 53, 55, 56, 57:
		return "毛毛雨"
	case 61, 63, 65, 66, 67:
		return "雨"
	case 71, 73, 75, 77:
		return "雪"
	case 80, 81, 82:
		return "阵雨"
	case 85, 86:
		return "阵雪"
	case 95, 96, 99:
		return "雷暴"
	default:
		return "天气代码 " + strconv.Itoa(code)
	}
}

func getJSON(rawURL string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := weatherHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("weather %d", res.StatusCode)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
