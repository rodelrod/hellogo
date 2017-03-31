package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var conf = readConfig()

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/weather/", weather)
	http.ListenAndServe(":8888", nil)
}

type Config struct {
	OpenWeatherMapApiKey     string
	WeatherUndergroundApiKey string
}

type weatherData struct {
	Name string `json:"name"`
	Main struct {
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

func readConfig() Config {
	var conf Config
	tomlData, err := ioutil.ReadFile("secrets.toml")
	if err != nil {
		panic(err)
	}

	if _, err := toml.Decode(string(tomlData), &conf); err != nil {
		fmt.Println(err)
	}
	return conf
}

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello!\n"))
}

func weather(w http.ResponseWriter, r *http.Request) {
	mw := multiWeatherProvider{
		openWeatherMap{},
		weatherUnderground{},
	}
	begin := time.Now()
	city := strings.SplitN(r.URL.Path, "/", 3)[2]
	temp, err := mw.temperature(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city": city,
		"temp": temp,
		"took": time.Since(begin).String(),
	})
}

/*
 *
 * WEATHER PROVIDERS
 *
 */

type weatherProvider interface {
	apiKey() string
	temperature(city string) (float64, error) // in Kelvin
}

/*
 * Open Weather Map
 */

type openWeatherMap struct{}

func (w openWeatherMap) apiKey() string {
	return conf.OpenWeatherMapApiKey
}

func (w openWeatherMap) temperature(city string) (float64, error) {

	// Call API
	apiUrl := fmt.Sprintf(
		"http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s",
		city, w.apiKey(),
	)
	log.Println(apiUrl)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	// Check if API call returns HTTP success
	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		return 0, errors.New(fmt.Sprintf("Got return code %d: %s", resp.StatusCode, body))
	}

	// Parse response JSON with temperature
	var d struct {
		Main struct {
			Kelvin float64 `json:"temp"`
		} `json:"main"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)
	return d.Main.Kelvin, nil
}

/*
 * Weather Underground
 */

type weatherUnderground struct{}

func (w weatherUnderground) apiKey() string {
	return conf.WeatherUndergroundApiKey
}

func (w weatherUnderground) temperature(city string) (float64, error) {

	// Call API
	apiUrl := fmt.Sprintf(
		"http://api.wunderground.com/api/%s/conditions/q/%s.json",
		w.apiKey(), city,
	)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	// Check if API call returns HTTP success
	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, err
		}
		return 0, errors.New(fmt.Sprintf(
			"Got return code %d: %s", resp.StatusCode, body,
		))
	}

	// Parse response JSON with temperature
	var d struct {
		Observation struct {
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	kelvin := d.Observation.Celsius + 273.15
	log.Printf("weatherUnderground: %s: %.2f", city, kelvin)
	return kelvin, nil
}

type multiWeatherProvider []weatherProvider

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	sum := 0.0

	for _, provider := range w {
		k, err := provider.temperature(city)
		if err != nil {
			return 0, err
		}
		sum += k
	}
	return sum / float64(len(w)), nil
}
