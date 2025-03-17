package service

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	sheet "github.com/nilemarezz/nana-batch-v2/client"
	"github.com/nilemarezz/nana-batch-v2/model"
	"google.golang.org/api/sheets/v4"
)

var exceptionShipping []string = []string{"ยังไม่ได้เก็บ", "ฝาก", "รอชำระ", "นัดรับ", "ส่งในญี่ปุ่น", "ตี้หาร"}

type OrderGroup struct {
	Address string
	Order   []model.DataSheet
}

func RunShippingFee(sheetName string) (bool, error) {

	fmt.Println("run command gen " + sheetName)

	srv, err := sheet.NewService()
	if err != nil {
		return false, fmt.Errorf("Unable to retrieve Sheets client: %v", err)
	}

	////////////////////// bussiness logic here

	// load spredSheetId from config
	err = godotenv.Load(".env")
	if err != nil {
		return false, fmt.Errorf("Error loading .env file")
	}
	spreadsheetId := os.Getenv("NANA_SHEET")

	mainData, err := getDataFromMainSheet(srv, spreadsheetId)
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	subDataSheet, err := getDataFromShippingFeeSheet(srv, spreadsheetId, sheetName)
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	for i := 1; i < len(subDataSheet); i++ {
		// fill data (address , shipping)
		timestamp := subDataSheet[i].Order_Timestamp
		product := subDataSheet[i].Product
		account := subDataSheet[i].Account
		for _, m := range mainData {
			if m.Account == account && m.Product == product && m.Order_Timestamp == timestamp {
				subDataSheet[i].Shipping = m.Shipping

				if m.Address == "" {
					subDataSheet[i].Address = "no_address"
				} else {
					subDataSheet[i].Address = m.Address
				}

			}
		}
	}

	sort.Slice(subDataSheet, func(i, j int) bool {
		return subDataSheet[i].Product < subDataSheet[j].Product
	})

	groupedByOrder := []*OrderGroup{}
	addressMap := make(map[string]*OrderGroup)

	for _, data := range subDataSheet {
		if group, exists := addressMap[data.Address]; exists {
			group.Order = append(group.Order, data)
		} else {
			newGroup := &OrderGroup{Address: data.Address, Order: []model.DataSheet{data}}
			groupedByOrder = append(groupedByOrder, newGroup)
			addressMap[data.Address] = newGroup
		}
	}

	writeFile(groupedByOrder, sheetName)

	fmt.Println("output " + sheetName + ".txt")

	return true, nil

}

func writeFile(subData []*OrderGroup, sheetName string) error {
	s := strings.Split(sheetName, " ")
	batchDate := s[1]
	layout := "2/1/06"
	// Parse the input date string
	date, err := time.Parse(layout, batchDate)
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return fmt.Errorf("error parsing date")
	}
	// Create or open the output file

	// now := time.Now()

	formattedNewDate := date.Format("02-01-2006")

	// // Format the time as dd-mm-yyyy
	// formattedTime := now.Format("02-01-2006")

	file, err := os.Create(formattedNewDate + "_ค่าส่ง" + ".txt")
	if err != nil {
		fmt.Println("Error creating file:", err)
		return nil
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	groupedByAccount := make(map[string][]model.DataSheet)

	for _, v := range subData {
		// line := fmt.Sprintf("%s - %s\n", account, strings.Join(products, ", "))
		// _, err := writer.WriteString(line)
		address := v.Address
		orders := v.Order

		if address == "no_address" {

			// group order by account
			for _, order := range orders {
				groupedByAccount[order.Account] = append(groupedByAccount[order.Account], order)
			}
		} else {

			countOrder := 0
			for _, order := range orders {
				shipping := order.Shipping
				// check skip flag
				if order.SkipFlag != "Y" {
					// check shipping status
					if isNotInSlice(shipping, exceptionShipping) {
						writer.WriteString(fmt.Sprintf("%v \t %v \n", order.Account, order.Product))
						countOrder++
					}
				}
			}

			if v.Order[0].Shipping != "flash" && countOrder != 0 {
				writer.WriteString(fmt.Sprintf("%v \n", v.Order[0].Shipping))
			}

			if countOrder != 0 {
				writer.WriteString(fmt.Sprintf("%v \n \n", address))
			}
		}
	}

	if len(groupedByAccount) != 0 {

		for _, orders := range groupedByAccount {
			countOrder := 0
			for _, order := range orders {
				shipping := order.Shipping
				// check skip flag
				if order.SkipFlag != "Y" {
					// check shipping status
					if isNotInSlice(shipping, exceptionShipping) {
						writer.WriteString(fmt.Sprintf("%v \t %v \n", order.Account, order.Product))
						countOrder++
					}
				}

			}

			if orders[0].Shipping != "flash" && countOrder != 0 {
				writer.WriteString(fmt.Sprintf("%v \n", orders[0].Shipping))
			}

			if countOrder != 0 {
				writer.WriteString(fmt.Sprintf("%v \n \n", "{ที่อยู่}"))
			}

		}
	}

	// Flush the buffer to ensure all data is written to file
	err = writer.Flush()
	if err != nil {
		fmt.Println("Error flushing buffer:", err)

	}

	return nil
}

func getDataFromMainSheet(srv *sheets.Service, spreadsheetId string) ([]model.DataSheet, error) {

	/// get data from main sheet
	dataMainSheet := []model.DataSheet{}

	// define sheetName
	mainSheetName := "รวมทั้งหมด"

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, mainSheetName).Do()
	if err != nil {
		log.Fatalf("%v", err)
	}

	if len(resp.Values) == 0 {
		return nil, errors.New("No data found.")
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
		data := model.DataSheet{
			Order_Timestamp: paddedRow[sheet.SheetTemplate["Timestamp"]].(string),
			Account:         paddedRow[sheet.SheetTemplate["@Twitter"]].(string),
			Product:         paddedRow[sheet.SheetTemplate["รายการสั่งซื้อ"]].(string),
			Shipping:        paddedRow[sheet.SheetTemplate["ขนส่ง"]].(string),
			Address:         paddedRow[sheet.SheetTemplate["ชื่อ-ที่อยู่-เบอร์โทร"]].(string),
			TrackingNo:      paddedRow[sheet.SheetTemplate["เลข Tracking"]].(string),
			SendDate:        paddedRow[sheet.SheetTemplate["รอบส่ง"]].(string),
		}
		// Order_Timestamp: paddedRow[sheet.SheetTemplate["Timestamp"]].(string),
		// 	Account:         paddedRow[sheet.SheetTemplate["@Twitter"]].(string),
		// 	Product:         paddedRow[sheet.SheetTemplate["รายการสั่งซื้อ"]].(string),
		// 	Shipping:        paddedRow[sheet.SheetTemplate["ขนส่ง"]].(string),
		// 	Address:         paddedRow[sheet.SheetTemplate["ชื่อ-ที่อยู่-เบอร์โทร"]].(string),
		// 	TrackingNo:      paddedRow[sheet.SheetTemplate["เลข Tracking"]].(string),
		// 	SendDate:        paddedRow[sheet.SheetTemplate["รอบส่ง"]].(string),

		dataMainSheet = append(dataMainSheet, data)
	}

	return dataMainSheet, nil
}

func getDataFromShippingFeeSheet(srv *sheets.Service, spreadsheetId string, sheetName string) ([]model.DataSheet, error) {
	// / get data from main sheet
	dataSubSheet := []model.DataSheet{}

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, sheetName).Do()
	if err != nil {
		log.Fatalf("%v", err)
	}

	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
		return nil, errors.New("no data found")
	}

	// Find the maximum number of columns in the sheet
	maxColumns := 15
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

		if paddedRow[15].(string) != "Y" {

			shipping_fee, _ := strconv.ParseFloat(paddedRow[11].(string), 64)
			express_fee, _ := strconv.ParseFloat(paddedRow[12].(string), 64)
			payment_fee, _ := strconv.ParseFloat(paddedRow[10].(string), 64)
			data := model.DataSheet{
				Order_Timestamp: paddedRow[0].(string),
				Account:         paddedRow[1].(string),
				Product:         paddedRow[4].(string),
				SkipFlag:        paddedRow[15].(string),
				Shipping_fee:    shipping_fee,
				Express_fee:     express_fee,
				Payment_fee:     payment_fee,
				Notice:          paddedRow[5].(string),
			}

			dataSubSheet = append(dataSubSheet, data)
		}
	}

	return dataSubSheet, nil
}

// Function to check if a string is not in slice
func isNotInSlice(target string, slice []string) bool {
	for _, s := range slice {
		if s == strings.Trim(target, " ") {
			return false // Found in slice
		}
	}
	return true // Not found in slice
}
