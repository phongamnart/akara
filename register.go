package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "github.com/lib/pq"
)

type ESP32Data struct {
	ChipID string `json:"chipid"`
	Token  string `json:"token"`
}

const (
	host     = "172.16.60.211"
	port     = 5432
	user     = "postgres"
	password = "akara"
	dbname   = "akara"
)

// function connect database
func dbConnect() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}

// function to check if chipID exists in the database
func isChipIDExists(chipID string) (bool, error) {
	db, err := dbConnect()
	if err != nil {
		return false, err
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM chip_data WHERE chipid = $1", chipID).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// function insert to database
func saveToDB(data *ESP32Data) error {
	db, err := dbConnect()
	if err != nil {
		return err
	}
	defer db.Close()

	currentTime := time.Now().Round(time.Second)

	exists, err := isChipIDExists(data.ChipID)
	if err != nil {
		return err
	}

	if exists {
		return errors.New("ChipID " + data.ChipID + " already exists in the database")
	}

	_, err = db.Exec("INSERT INTO chip_data (chipid, token, time) VALUES ($1, $2, $3)", data.ChipID, data.Token, currentTime)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	app := fiber.New()
	data := new(ESP32Data)

	//allow all method
	app.Use(func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		return c.Next()
	})

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Fatal(err)
	}

	//method save from html to postgres
	app.Post("/saveToPostgres", func(c *fiber.Ctx) error {
		if err := c.BodyParser(data); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}

		if err := saveToDB(data); err != nil {
			log.Println("Error saving to database:", err)
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		fmt.Printf("Saved ChipID: %s, Token: %s to postgres\n", data.ChipID, data.Token)
		return c.JSON(fiber.Map{"success": true})
	})

	//method send from esp32 to html
	app.Post("/register", func(c *fiber.Ctx) error {
		if err := c.BodyParser(data); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}

		if err != nil {
			return err
		}
		fmt.Printf("Received ChipID: %s, Token: %s\n", data.ChipID, data.Token)

		return c.Status(200).JSON(fiber.Map{"message": "Data received successfully"})
	})

	//method show from esp32 on html
	app.Get("/register", func(c *fiber.Ctx) error {
		var placeholderData *ESP32Data
		placeholderData = &ESP32Data{
			ChipID: data.ChipID,
			Token:  data.Token,
		}

		if err := tmpl.Execute(c.Response().BodyWriter(), placeholderData); err != nil {
			return c.Status(http.StatusInternalServerError).SendString("Internal Server Error")
		}

		c.Type("html")

		return nil
	})

	app.Listen(":9001")
}
