package services

import (
	"encoding/csv"
	_"io"
	"os"
	"path/filepath"
)

// SaveTextToCSV appends the provided text to data/text.csv.
func SaveTextToCSV(text string) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}
	file, err := os.OpenFile("data/text.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	return writer.Write([]string{text})
}

// SaveImageToFile saves the image data into data/images using the provided filename.
func SaveImageToFile(data []byte, filename string) error {
	if err := os.MkdirAll("data/images", 0755); err != nil {
		return err
	}
	filePath := filepath.Join("data/images", filename)
	return os.WriteFile(filePath, data, 0644)
}
