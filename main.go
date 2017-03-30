package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"net/http"
	"strings"
)

type Config struct {
	OpenWeatherMapApiKey     string
	WeatherUndergroundApiKey string
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

var conf = readConfig()

func main() {
	http.HandleFunc("/", hello)
	http.HandleFunc("/weather/", weather)
	http.ListenAndServe(":8888", nil)
}

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello!\n"))
}

func weather(w http.ResponseWriter, r *http.Request) {
	city := strings.SplitN(r.URL.Path, "/", 3)[2]
	data, err := query(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

type weatherData struct {
	Name string `json:"name"`
	Main struct {
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

func query(city string) (weatherData, error) {
	apiUrl := fmt.Sprintf(
		"http://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s",
		city, conf.OpenWeatherMapApiKey,
	)
	resp, err := http.Get(apiUrl)
	if err != nil {
		return weatherData{}, err
	}
	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return weatherData{}, err
		}
		return weatherData{}, errors.New(fmt.Sprintf("Got return code %d: %s", resp.StatusCode, body))
	}
	defer resp.Body.Close()
	var d weatherData
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		println("Error converting to json")
		return weatherData{}, err
	}
	fmt.Printf("%+v\n", d)
	return d, nil
}

type weatherProvider interface {
	temperature(city string) (float64, error)
}
