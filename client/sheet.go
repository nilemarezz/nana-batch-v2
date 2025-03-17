package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Credentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// init  map key is string, value is int
var SheetTemplate = map[string]int{}

func NewService() (*sheets.Service, error) {
	// Load the .env file
	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatalf("Error loading .env file")
	// }

	privateKey := os.Getenv("PRIVATE_KEY")
	clientEmail := os.Getenv("CLIENT_EMAIL")

	privateKey = strings.Replace(privateKey, "\\n", "\n", -1)

	creds := Credentials{
		Type:                    "service_account",
		PrivateKey:              privateKey,
		ClientEmail:             clientEmail,
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       fmt.Sprintf("https://www.googleapis.com/robot/v1/metadata/x509/%s", clientEmail),
	}

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		log.Printf("Error marshalling credentials: %v", err)
		return nil, err
	}

	ctx := context.Background()
	config, err := google.JWTConfigFromJSON(credsJSON, sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		log.Printf("Unable to parse client secret file to config: %v", err)
		return nil, err
	}

	client := config.Client(ctx)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Printf("Unable to retrieve Sheets client: %v", err)
		return nil, err
	}

	return srv, nil
}

// function to read data from template/Address_print.txt and get value[0] as key and value[1] as value and set it into SheetTemplate which text use , as delimiter
func ReadTemplate() {
	file, err := os.Open("./template/Sheet_Template.txt")
	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		s := strings.Split(line, ",")
		SheetTemplate[s[0]], err = strconv.Atoi(s[1])
		if err != nil {
			log.Fatalf("failed convert string to int: %s", err)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("failed reading file: %s", err)
	}

	fmt.Println(SheetTemplate)
}
