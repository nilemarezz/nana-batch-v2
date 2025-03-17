package main

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	sheet "github.com/nilemarezz/nana-batch-v2/client"
	"github.com/nilemarezz/nana-batch-v2/service"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	app := fiber.New()

	// Enable CORS for all routes
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Allow all origins
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
	}))

	sheet.ReadTemplate()

	err := godotenv.Load(".env")
	if err != nil {
		fmt.Errorf("Error loading .env file")
	}

	app.Post("/api/batch", func(c *fiber.Ctx) error {
		var result bool
		var err error
		// get header value authorization
		token := c.Get("authorization")

		// compare token bcrypt
		plainPassword := os.Getenv("NANA_PASSWORD")
		fmt.Println("plainPassword", plainPassword)
		fmt.Println("token", token)
		err = bcrypt.CompareHashAndPassword([]byte(token), []byte(plainPassword))
		if err != nil {
			fmt.Println(err)
			return c.JSON(fiber.Map{
				"message": "Unauthorized",
				"result":  false,
			})
		}

		// get request body
		payload := struct {
			SheetName string `json:"sheetName"`
			Runtype   string `json:"runType"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}

		sheetName := payload.SheetName
		runType := payload.Runtype

		fmt.Println("Sheet Name: ", sheetName)
		fmt.Println("Run Type: ", runType)

		driveUrl := "https://drive.google.com/drive/folders/"

		// sheetName = sheetName[:len(sheetName)-1] // Remove the newline character

		if runType == "2" {
			result, err = service.RunPledge(sheetName)
			driveUrl += os.Getenv("NANA_DRIVE_PLEDGE")
		} else if runType == "1" {
			result, err = service.RunShippingFeeTemplate(sheetName)
			driveUrl += os.Getenv("NANA_SHIPPING_FEE")
		} else if runType == "3" {
			result, err = service.PrintAddressTemplate(sheetName)
			driveUrl += os.Getenv("NANA_DRIVE_ADDRESS")
		}

		if err != nil {
			return c.JSON(fiber.Map{
				"message": err.Error(),
				"result":  result,
			})
		}

		return c.JSON(fiber.Map{
			"message": driveUrl,
			"result":  result,
		})

	})

	// Get the port from the environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "6001" // Default port if not set
	}

	app.Listen(":" + port)
}
