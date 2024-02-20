package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

const (
	host                = "172.16.60.211"
	port                = 5432
	user                = "postgres"
	password            = "akara"
	dbname              = "akara"
	DurationToAggregate = 1 * time.Minute
)

type Device struct {
	ChipID      string  `json:"chipid"`
	Token       string  `json:"token"`
}

type Data struct {
	ChipID      string  `json:"chipid"`
	Humidity    float32 `json:"humidity"`
	Temperature float32 `json:"temperature"`
}

type AggregatedData struct {
	HumiditySum    float32
	TemperatureSum float32
	DataCount      int
}

type BoardData struct {
    AggregatedData
    LastAggregationTime time.Time
}

var (
	db                  *sql.DB
	aggregatedData      AggregatedData
	lastAggregationTime time.Time
	boardData           sync.Map
	mutex               sync.Mutex
)

func connectToDB() error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	if err = db.Ping(); err != nil {
		return err
	}
	return nil
}

func storeDataInPostgres(chipID, token string) error {
	if chipIDExistsInPostgres(chipID) {
		fmt.Printf("ChipID %s already exists in the database\n", chipID)
		return nil
	}

	if err := connectToDB(); err != nil {
		return err
	}
	defer db.Close()

	_, err := db.Exec("INSERT INTO chip_data (chipid, token) VALUES ($1, $2)", chipID, token)
	if err != nil {
		return err
	}

	fmt.Println("Successfully stored chip data in the database")
	return nil
}

func chipIDExistsInPostgres(chipID string) bool {
	if err := connectToDB(); err != nil {
		return false
	}
	defer db.Close()

	var storedChipID string
	err := db.QueryRow("SELECT chipid FROM chip_data WHERE chipid = $1", chipID).Scan(&storedChipID)
	if err != nil {
		return false
	}

	return true
}

func handleESP32Data(w http.ResponseWriter, r *http.Request) {
	var chipID string
	contentLength := r.ContentLength
	body := make([]byte, contentLength)

	_, err := r.Body.Read(body)
	if err != nil && err != io.EOF {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
	}

	var data Device
	if err := json.Unmarshal(body, &data); err != nil {
			http.Error(w, "Unable to parse JSON data into the data structure", http.StatusBadRequest)
			return
	}

	chipID = data.ChipID

	fmt.Printf("ChipID %s not registered in database\n", chipID)
	w.WriteHeader(http.StatusOK)
}


func handleDHT(w http.ResponseWriter, r *http.Request) {
	contentLength := r.ContentLength
	body := make([]byte, contentLength)

	_, err := r.Body.Read(body)
	if err != nil && err != io.EOF {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	var data Data
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Unable to parse JSON data into the data structure", http.StatusBadRequest)
		return
	}

	chipID := data.ChipID
	humid := fmt.Sprintf("%.2f", data.Humidity)
	temp := fmt.Sprintf("%.2f", data.Temperature)

	mutex.Lock()
	defer mutex.Unlock()

	currentBoardInterface, ok := boardData.Load(chipID)
    var currentBoard *BoardData
    if ok {
        currentBoard = currentBoardInterface.(*BoardData)
    } else {
        currentBoard = &BoardData{}
        boardData.Store(chipID, currentBoard)
    }

	currentBoard.HumiditySum += data.Humidity
	currentBoard.TemperatureSum += data.Temperature
	currentBoard.DataCount++

	if chipIDExistsInPostgres(chipID) {
        currentTime := time.Now()
        fmt.Printf("ChipID: %s, Humidity: %s, Temperature: %s°\n", chipID, humid, temp)

        if currentTime.Sub(currentBoard.LastAggregationTime) >= DurationToAggregate {
            if currentBoard.DataCount > 0 {
                avgHumid := currentBoard.HumiditySum / float32(currentBoard.DataCount)
                avgTemp := currentBoard.TemperatureSum / float32(currentBoard.DataCount)

                fmt.Printf("Average data of ChipID: %s : Humidity=%.2f%% Temperature=%.2f°C\n", chipID, avgHumid, avgTemp)

                if err := storeWeatherData(chipID, avgHumid, avgTemp); err != nil {
                    fmt.Println("Error storing weather data:", err)
                    http.Error(w, "Error storing weather data", http.StatusInternalServerError)
                    return
                }
            }

            currentBoard = &BoardData{}
            currentBoard.LastAggregationTime = currentTime
            boardData.Store(chipID, currentBoard)
        }

        w.WriteHeader(http.StatusOK)
        return
    }
}

func storeWeatherData(chipID string, avgHumid, avgTemp float32) error {
	if err := connectToDB(); err != nil {
		return err
	}
	defer db.Close()

	currentTime := time.Now().Round(time.Second)

	_, err := db.Exec("INSERT INTO weather (chipid, humidity, temperature, time) VALUES ($1, ROUND($2::numeric, 2), ROUND($3::numeric, 2), $4)", chipID, avgHumid, avgTemp, currentTime)
	if err != nil {
		return err
	}

	fmt.Printf("ChipID: %s Successfully stored weather data in the database\n", chipID)

	return nil
}

func main() {
	http.HandleFunc("/esp32data", handleESP32Data)
	http.HandleFunc("/weather", handleDHT)

	fmt.Println("Server is running on port 9000...")
	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatal(err)
	}
}
