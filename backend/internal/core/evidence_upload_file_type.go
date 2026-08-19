package core

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

type evidenceUploadFileSpec struct {
	Extension    string
	EvidenceType string
	MIMEType     string
	FileKind     string
}

var evidenceImageUploadSpecs = map[string]evidenceUploadFileSpec{
	"image/jpeg": {Extension: ".jpg", EvidenceType: "photo", MIMEType: "image/jpeg", FileKind: "photo"},
	"image/png":  {Extension: ".png", EvidenceType: "photo", MIMEType: "image/png", FileKind: "photo"},
	"image/webp": {Extension: ".webp", EvidenceType: "photo", MIMEType: "image/webp", FileKind: "photo"},
	"image/gif":  {Extension: ".gif", EvidenceType: "photo", MIMEType: "image/gif", FileKind: "photo"},
}

var compoundFileSignature = []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}

func detectEvidenceUploadFile(source multipart.File, size int64, filename string) (evidenceUploadFileSpec, error) {
	if source == nil || size <= 0 {
		return evidenceUploadFileSpec{}, validationError("Tệp minh chứng không được rỗng")
	}

	header := make([]byte, 512)
	n, err := io.ReadFull(source, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return evidenceUploadFileSpec{}, err
	}
	header = header[:n]
	detectedMIME := http.DetectContentType(header)
	if spec, ok := evidenceImageUploadSpecs[detectedMIME]; ok {
		return spec, nil
	}
	if detectedMIME == "application/pdf" {
		return evidenceUploadFileSpec{Extension: ".pdf", EvidenceType: "document", MIMEType: "application/pdf", FileKind: "pdf"}, nil
	}

	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if bytes.HasPrefix(header, compoundFileSignature) {
		switch extension {
		case ".doc":
			return evidenceUploadFileSpec{Extension: ".doc", EvidenceType: "document", MIMEType: "application/msword", FileKind: "word"}, nil
		case ".xls":
			return evidenceUploadFileSpec{Extension: ".xls", EvidenceType: "document", MIMEType: "application/vnd.ms-excel", FileKind: "excel"}, nil
		default:
			return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Tệp Office dạng OLE cần phần mở rộng .doc hoặc .xls")
		}
	}

	if isZipHeader(header) || statusIn(detectedMIME, "application/zip", "application/x-zip-compressed") {
		return detectOpenXMLEvidenceUpload(source, size, filename, detectedMIME)
	}

	return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Chỉ hỗ trợ ảnh PNG/JPG/JPEG/WebP/GIF, PDF, Word và Excel")
}

func detectOpenXMLEvidenceUpload(source multipart.File, size int64, filename, detectedMIME string) (evidenceUploadFileSpec, error) {
	reader, err := zip.NewReader(source, size)
	if err != nil {
		return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Tệp Office Open XML không có cấu trúc ZIP hợp lệ")
	}
	if len(reader.File) > 4096 {
		return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Tệp Office Open XML có quá nhiều thành phần")
	}

	hasContentTypes := false
	hasWordDocument := false
	hasExcelWorkbook := false
	hasMacroProject := false
	for _, entry := range reader.File {
		name := strings.TrimPrefix(strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/")), "/")
		switch name {
		case "[content_types].xml":
			hasContentTypes = true
		case "word/document.xml":
			hasWordDocument = true
		case "xl/workbook.xml":
			hasExcelWorkbook = true
		case "word/vbaproject.bin", "xl/vbaproject.bin":
			hasMacroProject = true
		}
	}

	if !hasContentTypes || hasMacroProject || hasWordDocument == hasExcelWorkbook {
		reason := "Tệp Office Open XML không đúng cấu trúc DOCX/XLSX được hỗ trợ"
		if hasMacroProject {
			reason = "Tệp Office có macro không được hỗ trợ"
		}
		return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, reason)
	}

	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if hasWordDocument {
		if extension != ".docx" {
			return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Nội dung Word Open XML cần phần mở rộng .docx")
		}
		return evidenceUploadFileSpec{
			Extension:    ".docx",
			EvidenceType: "document",
			MIMEType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			FileKind:     "word",
		}, nil
	}
	if extension != ".xlsx" {
		return evidenceUploadFileSpec{}, unsupportedEvidenceUploadError(filename, detectedMIME, "Nội dung Excel Open XML cần phần mở rộng .xlsx")
	}
	return evidenceUploadFileSpec{
		Extension:    ".xlsx",
		EvidenceType: "document",
		MIMEType:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		FileKind:     "excel",
	}, nil
}

func unsupportedEvidenceUploadError(filename, detectedMIME, reason string) StatusError {
	return validationError("Định dạng minh chứng không được hỗ trợ", Record{
		"filename":           cleanUploadFilename(filename),
		"detected_mime_type": strings.TrimSpace(detectedMIME),
		"reason":             reason,
	})
}

func isZipHeader(header []byte) bool {
	if len(header) < 4 || header[0] != 'P' || header[1] != 'K' {
		return false
	}
	return (header[2] == 0x03 && header[3] == 0x04) ||
		(header[2] == 0x05 && header[3] == 0x06) ||
		(header[2] == 0x07 && header[3] == 0x08)
}
