package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func NewDriveService() (*drive.Service, error) {

	ctx := context.Background()
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	privateKey := os.Getenv("PRIVATE_KEY")
	clientEmail := os.Getenv("CLIENT_EMAIL")

	if clientEmail == "" || privateKey == "" {
		log.Fatalf("CLIENT_EMAIL or PRIVATE_KEY environment variable is not set")
	}

	// Clean up the PRIVATE_KEY by replacing \n with actual newlines
	privateKey = strings.Replace(privateKey, "\\n", "\n", -1)
	// Configure the JWT config
	config := &jwt.Config{
		Email:      clientEmail,
		PrivateKey: []byte(privateKey),
		Scopes:     []string{drive.DriveFileScope},
		TokenURL:   "https://oauth2.googleapis.com/token",
	}

	// Create the Drive service
	srv, err := drive.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve Drive client: %v", err)
	}

	return srv, nil
}

func UploadFile(srv *drive.Service, file *os.File, driveId string) (string, error) {
	folderID := driveId // Replace with your actual folder ID

	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}

	fileMetadata := &drive.File{
		Name:    fileInfo.Name(),
		Parents: []string{folderID}, // Specify the folder ID here
	}

	driveFile, err := srv.Files.Create(fileMetadata).Media(file).Do()
	if err != nil {
		return "", err
	}

	// Make the file public
	_, err = srv.Permissions.Create(driveFile.Id, &drive.Permission{
		Role: "reader",
		Type: "anyone",
	}).Do()

	if err != nil {
		return "", err
	}

	return driveFile.Id, nil
}
