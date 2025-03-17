package service

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	sheet "github.com/nilemarezz/nana-batch-v2/client"
	"github.com/nilemarezz/nana-batch-v2/model"
	"google.golang.org/api/drive/v3"
)

func RunShippingFeeTemplate(sheetName string) (bool, error) {
	fmt.Println("run command gen " + sheetName)

	s := strings.Split(sheetName, " ")
	batchDate := s[1]

	// Define custom layout with leading zeros
	layout := "2/1/06"

	// Parse the input date string
	date, err := time.Parse(layout, batchDate)
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return false, fmt.Errorf("error parsing date")
	}

	formattedNewDate := date.Format("02-01-2006")

	srv, err := sheet.NewService()
	if err != nil {
		return false, fmt.Errorf("Unable to retrieve Sheets client: %v", err)
	}

	////////////////////// bussiness logic here

	// // load spredSheetId from config
	// err = godotenv.Load(".env")
	// if err != nil {
	// 	return false, fmt.Errorf("Error loading .env file")
	// }
	spreadsheetId := os.Getenv("NANA_SHEET")

	subDataSheet, err := getDataFromShippingFeeSheet(srv, spreadsheetId, sheetName)
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	// fill express fee
	for i, v := range subDataSheet {
		if v.Express_fee == 0 {
			subDataSheet[i].Express_fee = FindExpressFeeByAccount(v.Account, subDataSheet)
		}
	}

	shippingExpressFeeList := []model.DataSheet{}
	shippingExpressList := []model.DataSheet{}
	expressList := []model.DataSheet{}
	expressFeeList := []model.DataSheet{}

	groupedByAccount := make(map[string][]model.DataSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	for _, v := range groupedByAccount {
		shipping_fee := 0
		express_fee := 0
		payment_fee := 0

		for _, order := range v {
			shipping_fee = shipping_fee + int(order.Shipping_fee)
			express_fee = express_fee + int(order.Express_fee)
			payment_fee = payment_fee + int(order.Payment_fee)
		}
		// express_fee := v.Express_fee
		// payment_fee := v.Payment_fee

		if shipping_fee != 0 && express_fee != 0 && payment_fee != 0 {
			shippingExpressFeeList = append(shippingExpressFeeList, v...)
		} else if express_fee != 0 && shipping_fee != 0 {
			shippingExpressList = append(shippingExpressList, v...)
		} else if express_fee != 0 && payment_fee != 0 {
			expressFeeList = append(expressFeeList, v...)
		} else {
			expressList = append(expressList, v...)
		}
	}

	// INIT DRIVE
	driveService, err := sheet.NewDriveService()
	if err != nil {
		return false, fmt.Errorf("unable to retrieve Drive client: %v", err)
	}

	writeFileShippingExpressFeeList(shippingExpressFeeList, formattedNewDate, driveService)
	writeFileExpressList(expressList, formattedNewDate, driveService)
	writeFileShippingExpressList(shippingExpressList, formattedNewDate, driveService)
	writeExpressFeeList(expressFeeList, formattedNewDate, driveService)
	return true, nil
}

func writeExpressFeeList(subDataSheet []model.DataSheet, formattedNewDate string, driveService *drive.Service) error {
	// get date now string
	now := time.Now()
	dateNowString := now.Format("02-01-2006")

	file, err := os.Create(dateNowString + "_" + formattedNewDate + "_ค่าส่ง_ค่าส่ง+ยอดรอชำระ" + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file:", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// group data first
	groupedByAccount := make(map[string][]model.DataSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	template_expressFee, err := os.ReadFile("./template/Template_expressFee.txt")
	if err != nil {
		return fmt.Errorf("Error reading file:", err)

	}

	for account, v := range groupedByAccount {
		ps := ""
		total_remain := 0.0
		express := v[0].Express_fee
		total := 0.0

		// product
		for i, o := range v {
			ps = ps + strconv.Itoa(i+1) + ". " + o.Product + " \n"
			total = total + o.Payment_fee
			total_remain = total_remain + o.Payment_fee
		}
		total = total + express
		s := strings.Replace(string(template_expressFee), "{products}", ps, 1)
		s = strings.Replace(s, "{total_remain}", fmt.Sprintf("%.2f", total_remain), 1)
		s = strings.Replace(s, "{express}", fmt.Sprintf("%.2f", express), 1)
		s = strings.Replace(s, "{total}", fmt.Sprintf("%.2f", total), 1)

		writer.WriteString(account)
		writer.WriteString(s)
		writer.WriteString("\n")
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
	_, err = sheet.UploadFile(driveService, file, os.Getenv("NANA_SHIPPING_FEE"))
	if err != nil {
		return fmt.Errorf("Error uploading file:  %v", err)
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return fmt.Errorf("Error deleting file:  %v", err)
	}

	return nil
}

func writeFileShippingExpressList(subDataSheet []model.DataSheet, formattedNewDate string, driveService *drive.Service) error {

	now := time.Now()
	dateNowString := now.Format("02-01-2006")

	file, err := os.Create(dateNowString + "_" + formattedNewDate + "_ค่าส่ง_ค่าส่ง+ค่าชิป" + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file:", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// group data first
	groupedByAccount := make(map[string][]model.DataSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	template_shippingExpress, err := os.ReadFile("./template/Template_shippingExpress.txt")
	if err != nil {
		return fmt.Errorf("Error reading file:", err)
	}

	for account, v := range groupedByAccount {
		ps := ""
		total_shipping := 0.0
		express := v[0].Express_fee
		total := 0.0

		// product
		for i, o := range v {
			ps = ps + strconv.Itoa(i+1) + ". " + o.Product + " \n"
			total = total + o.Shipping_fee
			total_shipping = total_shipping + o.Shipping_fee

		}
		total = total + express
		s := strings.Replace(string(template_shippingExpress), "{products}", ps, 1)
		s = strings.Replace(s, "{total_shipping}", fmt.Sprintf("%.2f", total_shipping), 1)
		s = strings.Replace(s, "{express}", fmt.Sprintf("%.2f", express), 1)
		s = strings.Replace(s, "{total}", fmt.Sprintf("%.2f", total), 1)

		writer.WriteString(account)
		writer.WriteString(s)
		writer.WriteString("\n")
		writer.WriteString("---------------------------------------------")
		writer.WriteString("\n")
	}

	// Flush the buffer to ensure all data is written to file
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Error flushing buffer:", err)
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
	_, err = sheet.UploadFile(driveService, file, os.Getenv("NANA_SHIPPING_FEE"))
	if err != nil {
		return fmt.Errorf("Error uploading file:  %v", err)
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return fmt.Errorf("Error deleting file:  %v", err)
	}

	return nil
}

func writeFileShippingExpressFeeList(subDataSheet []model.DataSheet, formattedNewDate string, driveService *drive.Service) error {

	now := time.Now()
	dateNowString := now.Format("02-01-2006")

	file, err := os.Create(dateNowString + "_" + formattedNewDate + "_ค่าส่ง_ค่าส่ง+ค่าชิป+ค่ามัดจำ" + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file:", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// group data first
	groupedByAccount := make(map[string][]model.DataSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	template_shippingExpressFee, err := os.ReadFile("./template/Template_shippingExpressFee.txt")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}

	for account, v := range groupedByAccount {
		ps := ""
		total_remain := 0.0
		total_shipping := 0.0
		express := v[0].Express_fee
		total := 0.0

		// product
		for i, o := range v {
			ps = ps + strconv.Itoa(i+1) + ". " + o.Product + " \n"
			total = total + o.Payment_fee + o.Shipping_fee
			total_shipping = total_shipping + o.Shipping_fee
			total_remain = total_remain + o.Payment_fee
		}
		total = total + express
		s := strings.Replace(string(template_shippingExpressFee), "{products}", ps, 1)
		s = strings.Replace(s, "{total_remain}", fmt.Sprintf("%.2f", total_remain), 1)
		s = strings.Replace(s, "{total_shipping}", fmt.Sprintf("%.2f", total_shipping), 1)
		s = strings.Replace(s, "{express}", fmt.Sprintf("%.2f", express), 1)
		s = strings.Replace(s, "{total}", fmt.Sprintf("%.2f", total), 1)

		writer.WriteString(account)
		writer.WriteString(s)
		writer.WriteString("\n")
		writer.WriteString("---------------------------------------------")
		writer.WriteString("\n")
	}

	// Flush the buffer to ensure all data is written to file
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Error flushing buffer:", err)
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
	_, err = sheet.UploadFile(driveService, file, os.Getenv("NANA_SHIPPING_FEE"))
	if err != nil {
		return fmt.Errorf("Error uploading file:  %v", err)
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return fmt.Errorf("Error deleting file:  %v", err)
	}

	return nil
}

func writeFileExpressList(subDataSheet []model.DataSheet, formattedNewDate string, driveService *drive.Service) error {

	now := time.Now()
	dateNowString := now.Format("02-01-2006")

	file, err := os.Create(dateNowString + "_" + formattedNewDate + "_ค่าส่ง_ส่งอย่างเดียว" + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file:", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// group data first
	groupedByAccount := make(map[string][]model.DataSheet)
	for _, order := range subDataSheet {
		groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
	}

	template_express, err := os.ReadFile("./template/Template_express.txt")
	if err != nil {
		return fmt.Errorf("Error reading file:", err)
	}

	for account, v := range groupedByAccount {
		ps := ""
		express := v[0].Express_fee

		// product
		for i, o := range v {
			ps = ps + strconv.Itoa(i+1) + ". " + o.Product + " \n"
		}
		s := strings.Replace(string(template_express), "{products}", ps, 1)
		s = strings.Replace(s, "{express}", fmt.Sprintf("%.2f", express), 1)

		writer.WriteString(account)
		writer.WriteString(s)
		writer.WriteString("\n")
		writer.WriteString("---------------------------------------------")
		writer.WriteString("\n")
	}

	// Flush the buffer to ensure all data is written to file
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("Error flushing buffer:", err)
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
	_, err = sheet.UploadFile(driveService, file, os.Getenv("NANA_SHIPPING_FEE"))
	if err != nil {
		return fmt.Errorf("Error uploading file:  %v", err)
	}

	// delete file
	err = os.Remove(file.Name())
	if err != nil {
		return fmt.Errorf("Error deleting file:  %v", err)
	}

	return nil

}

func FindExpressFeeByAccount(acc string, m []model.DataSheet) float64 {
	e := 0.0

	for _, v := range m {
		if v.Account == acc {
			e = v.Express_fee
			break
		}
	}

	return e
}
