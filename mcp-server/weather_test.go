package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCityFromText(t *testing.T) {
	cases := map[string]string{
		"北京今天天气怎么样":       "北京",
		"今天上海天气":          "上海",
		"查询杭州的天气":         "杭州",
		"天气：广州":           "广州",
		"Beijing weather": "Beijing",
	}
	for question, want := range cases {
		if got := cityFromText(question); got != want {
			t.Fatalf("%s => %q, want %s", question, got, want)
		}
	}
	if cityFromText("这道方程怎么去括号") != "" {
		t.Fatal("unrelated question should not yield a city")
	}
}

func TestWeatherQueryUsesLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "search"):
			_, _ = w.Write([]byte(`{"results":[{"name":"北京","latitude":39.9,"longitude":116.4}]}`))
		default:
			_, _ = w.Write([]byte(`{"current":{"temperature_2m":18.5,"relative_humidity_2m":40,"weather_code":2,"wind_speed_10m":11}}`))
		}
	}))
	defer srv.Close()
	oldGeo, oldCast, oldClient := geoEndpoint, forecastEndpoint, weatherHTTP
	geoEndpoint = srv.URL + "/search"
	forecastEndpoint = srv.URL + "/forecast"
	weatherHTTP = srv.Client()
	t.Cleanup(func() {
		geoEndpoint, forecastEndpoint, weatherHTTP = oldGeo, oldCast, oldClient
	})

	got := weatherQuery("北京今天天气怎么样", nil)
	if !strings.Contains(got, "北京") || !strings.Contains(got, "18.5") || !strings.Contains(got, "多云") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(weatherQuery("这道方程怎么去括号", nil), "18.5") {
		t.Fatal("missing city should not invent a temperature")
	}
	explicit := weatherQuery("今天天气怎么样", map[string]any{"city": "北京"})
	if !strings.Contains(explicit, "北京") || !strings.Contains(explicit, "18.5") {
		t.Fatalf("%s", explicit)
	}
}
