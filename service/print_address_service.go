package service

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"

	sheet "github.com/nilemarezz/nana-batch-v2/client"
	"github.com/nilemarezz/nana-batch-v2/model"
)

func PrintAddressTemplate(sendDate string) (bool, error) {

	srv, err := sheet.NewService()
	if err != nil {
		return false, fmt.Errorf("Unable to retrieve Sheets client: %v", err)
	}

	spreadsheetId := os.Getenv("NANA_SHEET")

	mainData, err := getDataFromMainSheet(srv, spreadsheetId)
	fmt.Println("mainData : ", len(mainData))
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	// loop mainData
	filterData := []model.DataSheet{}
	for _, v := range mainData {
		if (v.Shipping != "ยังไม่ได้เก็บ" && v.Shipping != "รอชำระค่าส่ง" && v.Shipping != "ตี้หาร" && v.Shipping != "ฝาก") && v.TrackingNo == "" && v.SendDate == sendDate {
			filterData = append(filterData, v)
		}
	}

	fmt.Println("filterData : ", len(filterData))

	// Sort filterData by product in descending order
	sort.SliceStable(filterData, func(i, j int) bool {
		return filterData[j].Product > filterData[i].Product
	})

	// Group filterData by account
	groupedData := make(map[string][]model.DataSheet)
	for _, data := range filterData {
		groupedData[data.Account] = append(groupedData[data.Account], data)
	}

	// Sort groupedData by the first product in each group in descending order
	groupedDataSorted := make([][]model.DataSheet, 0, len(groupedData))
	for _, data := range groupedData {
		groupedDataSorted = append(groupedDataSorted, data)
	}
	sort.SliceStable(groupedDataSorted, func(i, j int) bool {
		return groupedDataSorted[i][0].Product > groupedDataSorted[j][0].Product
	})

	// init new drive
	driveSrv, err := sheet.NewDriveService()
	if err != nil {
		return false, fmt.Errorf("Unable to retrieve Drive client: %v", err)
	}

	err = writeFileAddress(false, groupedDataSorted, driveSrv)
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	err = writeFileAddress(true, groupedDataSorted, driveSrv)
	if err != nil {
		return false, fmt.Errorf(" %v", err)
	}

	return true, nil
}

func writeFileAddress(isPrint bool, groupedDataSorted [][]model.DataSheet, driveSrv *drive.Service) error {
	fmt.Println("write file address : ", len(groupedDataSorted))

	// get current date string
	date := time.Now().Format("2006-01-02")
	fileName := date + "_รายชื่อจัดส่ง"
	if isPrint {
		fileName = date + "_รายชื่อจัดส่ง_print"
	}

	file, err := os.Create(fileName + ".txt")
	if err != nil {
		return fmt.Errorf("Error creating file: %v", err)

	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	template_address, err := os.ReadFile("./template/Address.txt")
	if err != nil {
		return fmt.Errorf("Error reading file: %v", err)
	}

	template_address_print, err := os.ReadFile("./template/Address_print.txt")
	if err != nil {
		return fmt.Errorf("Error reading file: %v", err)
	}

	// print result
	for _, data := range groupedDataSorted {
		acc_pro := ""
		address := ""
		for _, d := range data {

			if d.Address != "" && address == "" {
				address += d.Address
			}
			acc_pro = acc_pro + d.Account + "\t" + d.Product

			// if not the last loop , add + " \n" after
			if d != data[len(data)-1] {
				acc_pro = acc_pro + " \n"
			}

		}
		if !isPrint {
			s := strings.Replace(string(template_address), "{account_product}", acc_pro, 1)

			s = strings.Replace(s, "{shipping}", data[0].Shipping, 1)

			// find some address
			s = strings.Replace(s, "{address}", address, 1)

			writer.WriteString(s)
			writer.WriteString("\n")
		} else {
			s := strings.Replace(string(template_address_print), "{shipping}", data[0].Shipping, 1)

			// find some address
			s = strings.Replace(s, "{address}", address, 1)

			writer.WriteString(s)
			writer.WriteString("\n")
		}
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

	// upload file to google drive
	_, err = sheet.UploadFile(driveSrv, file, os.Getenv("NANA_DRIVE_ADDRESS"))
	if err != nil {
		return fmt.Errorf("Error uploading file: %v", err)
	}

	// delete file
	err = os.Remove(fileName + ".txt")
	if err != nil {
		return fmt.Errorf("Error deleting file: %v", err)
	}

	return nil
}
