package service

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	sheet "github.com/nilemarezz/nana-batch-v2/client"
	"github.com/nilemarezz/nana-batch-v2/model"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
)

func RunPledge(sheetName string) (bool, error) {

	fmt.Println("run command gen " + sheetName)

	s := strings.Split(sheetName, " ")
	batchDate := s[1]

	srv, err := sheet.NewService()
	if err != nil {
		return false, fmt.Errorf("unable to retrieve Sheets client: %v", err)
	}

	////////////////////// bussiness logic here

	// // load spredSheetId from config
	// err = godotenv.Load(".env")
	// if err != nil {
	// 	return false, fmt.Errorf("Error loading .env file")
	// }
	spreadsheetId := os.Getenv("NANA_SHEET")

	subDataSheet, err := getDataFromPledgeSheet(srv, spreadsheetId, sheetName)
	if err != nil {
		fmt.Println("error when get data")
		return false, fmt.Errorf(" %v", err)
	}

	groupedByAccount := make(map[string][]model.PledgeSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	// new drive
	driveService, err := sheet.NewDriveService()
	if err != nil {
		return false, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}
	writeXTemplateFile(batchDate, groupedByAccount, driveService)
	fmt.Println("output file : " + batchDate + "_มัดจำ" + ".txt")

	return true, nil
}

func writeXTemplateFile(batchDate string, groupedByAccount map[string][]model.PledgeSheet, driveService *drive.Service) error {

	fmt.Println(batchDate)

	// Define custom layout with leading zeros
	layout := "2/1/06"

	// Parse the input date string
	date, err := time.Parse(layout, batchDate)
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return fmt.Errorf("error parsing date")
	}

	// Add 7 days to the parsed date
	newDate := date.AddDate(0, 0, 7)

	// Format the new date back to d/m/yy
	formattedNewDate := newDate.Format("2/1/06")

	file, err := os.Create(date.Format("02-01-2006") + "_มัดจำ" + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file:", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	template_pledge, err := os.ReadFile("./template/Template_pledge.txt")
	if err != nil {
		return fmt.Errorf("Error reading file:", err)
	}

	for account, order := range groupedByAccount {
		s := strings.Replace(string(template_pledge), "{time_end}", batchDate, 1)
		s = strings.Replace(s, "{pay_until}", formattedNewDate, 1)

		total := 0.0
		ps := ""

		// fmt.Println(account)
		// fmt.Println(batchDate)
		// fmt.Println(formattedNewDate)
		for i, o := range order {
			ps = ps + strconv.Itoa(i+1) + ". " + o.Product + " " + o.Notice + " \n"
			total = total + o.Total
		}

		s = strings.Replace(s, "{products}", ps, 1)
		s = strings.Replace(s, "{total}", fmt.Sprintf("%.2f", total), 1)

		writer.WriteString(account)
		writer.WriteString(s)
		writer.WriteString("---------------------------------------------")
		writer.WriteString("\n")
	}

	// Flush the buffer to ensure all data is written to file
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Error flushing buffer: %v", err)

	}

	// Close the file to ensure all data is written
	err = file.Close()
	if err != nil {
		return fmt.Errorf("Error closing file: %v", err)
	}

	// Reopen the file for reading
	file, err = os.Open(file.Name())
	if err != nil {
		return fmt.Errorf("Error reopening file: %v", err)
	}
	defer file.Close()

	// Upload the file to Google Drive
	_, err = sheet.UploadFile(driveService, file, os.Getenv("NANA_DRIVE_PLEDGE"))
	if err != nil {
		return fmt.Errorf("Error uploading file: %v", err)
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return fmt.Errorf("Error deleting file: %v", err)
	}

	return nil
}

func getDataFromPledgeSheet(srv *sheets.Service, spreadsheetId string, sheetName string) ([]model.PledgeSheet, error) {
	dataPledgeSheet := []model.PledgeSheet{}

	fmt.Println("Getting data from sheet: "+spreadsheetId, sheetName)

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetName).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return nil, errors.New("no data found")
	}

	// Find the maximum number of columns in the sheet
	maxColumns := 17
	for _, row := range resp.Values {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}

	// Print data from the sheet with empty cells filled with ""
	for _, row := range resp.Values {
		paddedRow := make([]interface{}, maxColumns)
		for j := range paddedRow {
			if j < len(row) {
				paddedRow[j] = row[j]
			} else {
				paddedRow[j] = ""
			}
		}

		feetFloat, _ := strconv.ParseFloat(paddedRow[10].(string), 64)
		data := model.PledgeSheet{
			Order_Timestamp: paddedRow[0].(string),
			Account:         paddedRow[1].(string),
			Product:         paddedRow[4].(string),
			Total:           feetFloat,
			Notice:          paddedRow[5].(string),
		}

		dataPledgeSheet = append(dataPledgeSheet, data)
	}

	return dataPledgeSheet, nil
}
