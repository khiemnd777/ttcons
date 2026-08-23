package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	dataPackageFormatXLSX = "xlsx"
	dataPackageFormatJSON = "json"
	dataPackageVersionV1  = "1"
	exportJSONContentType = "application/json; charset=utf-8"
)

func normalizeDataPackageFormat(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "excel" {
		value = dataPackageFormatXLSX
	}
	if value != dataPackageFormatXLSX && value != dataPackageFormatJSON {
		return "", validationError("Định dạng gói dữ liệu không hợp lệ", Record{"format": value, "allowed": []string{dataPackageFormatXLSX, dataPackageFormatJSON}})
	}
	return value, nil
}

func normalizeDataPackageImportSource(sourceFormat, packageVersion, workbookVersion, filename string) (string, string, error) {
	format, err := normalizeDataPackageFormat(sourceFormat)
	if err != nil {
		return "", "", err
	}
	packageVersion = strings.TrimSpace(packageVersion)
	workbookVersion = strings.TrimSpace(workbookVersion)
	if packageVersion != "" && workbookVersion != "" && packageVersion != workbookVersion {
		return "", "", validationError("Phiên bản package và workbook không khớp", Record{"field": "package_version"})
	}
	if format == dataPackageFormatJSON && packageVersion == "" {
		return "", "", validationError("Gói JSON thiếu package_version", Record{"field": "package_version"})
	}
	version := packageVersion
	if version == "" {
		version = workbookVersion
	}
	if version != dataPackageVersionV1 {
		return "", "", validationError("Phiên bản gói dữ liệu không được hỗ trợ. Vui lòng tải lại file mẫu mới", Record{"field": "package_version", "expected": dataPackageVersionV1})
	}
	filename = strings.TrimSpace(filename)
	if filename == "" || len([]rune(filename)) > 255 {
		return "", "", validationError("Tên tệp import không hợp lệ", Record{"field": "source_filename"})
	}
	expectedExtension := "." + format
	if strings.ToLower(filepath.Ext(filename)) != expectedExtension {
		return "", "", validationError(fmt.Sprintf("Định dạng đã chọn chỉ chấp nhận tệp %s", expectedExtension), Record{"field": "source_filename", "source_format": format})
	}
	return format, version, nil
}

func marshalDataPackageJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateDataPackageRecordKeys(field string, rows []Record, allowed ...string) error {
	allowedKeys := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = true
	}
	for index, row := range rows {
		for key := range row {
			if !allowedKeys[key] {
				return validationError("Gói dữ liệu chứa trường không được hỗ trợ", Record{"field": field, "row_number": index + 2, "key": key})
			}
		}
	}
	return nil
}

func decodeStrictJSON(body []byte, target any) error {
	if err := validateJSONTokens(body); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func validateJSONTokens(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := validateJSONTokenValue(decoder, 0); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func validateJSONTokenValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON vượt quá độ sâu cho phép")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key không hợp lệ")
			}
			if keys[key] {
				return fmt.Errorf("JSON có key bị trùng: %s", key)
			}
			if key == "__proto__" || key == "prototype" || key == "constructor" {
				return fmt.Errorf("JSON chứa key không được phép: %s", key)
			}
			keys[key] = true
			if err := validateJSONTokenValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("JSON object chưa đóng hợp lệ")
		}
	case '[':
		for decoder.More() {
			if err := validateJSONTokenValue(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("JSON array chưa đóng hợp lệ")
		}
	default:
		return errors.New("JSON delimiter không hợp lệ")
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("JSON chỉ được chứa một document")
}
