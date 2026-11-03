package ibge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type sheetData struct {
	Rows [][]string
}

func readFirstSheetXLSX(data []byte) (sheetData, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return sheetData{}, err
	}

	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}

	sharedStrings, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return sheetData{}, err
	}
	sheetFile, ok := files["xl/worksheets/sheet1.xml"]
	if !ok {
		return sheetData{}, fmt.Errorf("sheet1.xml não encontrada no xlsx")
	}

	content, err := readZipFile(sheetFile)
	if err != nil {
		return sheetData{}, err
	}

	type cell struct {
		Ref   string `xml:"r,attr"`
		Type  string `xml:"t,attr"`
		Value string `xml:"v"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}
	type worksheet struct {
		Rows []row `xml:"sheetData>row"`
	}

	var ws worksheet
	if err := xml.Unmarshal(content, &ws); err != nil {
		return sheetData{}, err
	}

	rows := make([][]string, 0, len(ws.Rows))
	for _, rawRow := range ws.Rows {
		width := 0
		for _, rawCell := range rawRow.Cells {
			idx := cellColumnIndex(rawCell.Ref)
			if idx+1 > width {
				width = idx + 1
			}
		}
		if width == 0 {
			continue
		}
		values := make([]string, width)
		for _, rawCell := range rawRow.Cells {
			value := rawCell.Value
			if rawCell.Type == "s" {
				sharedIndex, err := strconv.Atoi(value)
				if err == nil && sharedIndex >= 0 && sharedIndex < len(sharedStrings) {
					value = sharedStrings[sharedIndex]
				}
			}
			values[cellColumnIndex(rawCell.Ref)] = strings.TrimSpace(value)
		}
		rows = append(rows, values)
	}

	return sheetData{Rows: rows}, nil
}

func readSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	content, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	type si struct {
		Texts []string `xml:"t"`
	}
	type sst struct {
		Items []si `xml:"si"`
	}
	var parsed sst
	if err := xml.Unmarshal(content, &parsed); err != nil {
		return nil, err
	}
	values := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		values = append(values, strings.Join(item.Texts, ""))
	}
	return values, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	opened, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer opened.Close()
	return io.ReadAll(opened)
}

func cellColumnIndex(ref string) int {
	letters := regexp.MustCompile(`[A-Z]+`).FindString(strings.ToUpper(ref))
	index := 0
	for _, char := range letters {
		index = index*26 + int(char-'A'+1)
	}
	if index == 0 {
		return 0
	}
	return index - 1
}

func parseFloat(value string) (float64, error) {
	clean := strings.TrimSpace(value)
	clean = strings.ReplaceAll(clean, ".", ".")
	clean = strings.ReplaceAll(clean, ",", ".")
	return strconv.ParseFloat(clean, 64)
}

func parseInt(value string) (int, error) {
	floatValue, err := parseFloat(value)
	if err != nil {
		return 0, err
	}
	return int(floatValue), nil
}
