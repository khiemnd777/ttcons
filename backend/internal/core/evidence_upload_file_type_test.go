package core

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
)

type memoryMultipartFile struct {
	*bytes.Reader
}

func (memoryMultipartFile) Close() error { return nil }

func TestDetectEvidenceUploadFileSupportedKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		filename     string
		contents     []byte
		wantKind     string
		wantType     string
		wantMIMEType string
	}{
		{name: "jpeg", filename: "site.jpeg", contents: []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}, wantKind: "photo", wantType: "photo", wantMIMEType: "image/jpeg"},
		{name: "png", filename: "site.png", contents: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, wantKind: "photo", wantType: "photo", wantMIMEType: "image/png"},
		{name: "webp", filename: "site.webp", contents: []byte{'R', 'I', 'F', 'F', 0x08, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '}, wantKind: "photo", wantType: "photo", wantMIMEType: "image/webp"},
		{name: "gif", filename: "site.gif", contents: []byte("GIF89a\x01\x00\x01\x00"), wantKind: "photo", wantType: "photo", wantMIMEType: "image/gif"},
		{name: "pdf", filename: "survey.pdf", contents: []byte("%PDF-1.7\n1 0 obj\n"), wantKind: "pdf", wantType: "document", wantMIMEType: "application/pdf"},
		{name: "doc", filename: "survey.doc", contents: append(append([]byte{}, compoundFileSignature...), make([]byte, 512)...), wantKind: "word", wantType: "document", wantMIMEType: "application/msword"},
		{name: "docx", filename: "survey.docx", contents: openXMLTestFile(t, "word/document.xml", false), wantKind: "word", wantType: "document", wantMIMEType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{name: "xls", filename: "survey.xls", contents: append(append([]byte{}, compoundFileSignature...), make([]byte, 512)...), wantKind: "excel", wantType: "document", wantMIMEType: "application/vnd.ms-excel"},
		{name: "xlsx", filename: "survey.xlsx", contents: openXMLTestFile(t, "xl/workbook.xml", false), wantKind: "excel", wantType: "document", wantMIMEType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := detectEvidenceUploadFile(memoryMultipartFile{bytes.NewReader(tt.contents)}, int64(len(tt.contents)), tt.filename)
			if err != nil {
				t.Fatalf("detectEvidenceUploadFile() error = %v", err)
			}
			if spec.FileKind != tt.wantKind || spec.EvidenceType != tt.wantType || spec.MIMEType != tt.wantMIMEType {
				t.Fatalf("detectEvidenceUploadFile() = %#v, want kind=%q evidence_type=%q mime=%q", spec, tt.wantKind, tt.wantType, tt.wantMIMEType)
			}
		})
	}
}

func TestDetectEvidenceUploadFileRejectsSpoofedOrUnsupportedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		contents []byte
	}{
		{name: "text renamed as pdf", filename: "survey.pdf", contents: []byte("not a pdf")},
		{name: "svg", filename: "site.svg", contents: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		{name: "generic zip renamed as docx", filename: "survey.docx", contents: openXMLTestFile(t, "notes/readme.xml", false)},
		{name: "word container with xlsx extension", filename: "survey.xlsx", contents: openXMLTestFile(t, "word/document.xml", false)},
		{name: "macro enabled word", filename: "survey.docm", contents: openXMLTestFile(t, "word/document.xml", true)},
		{name: "ole without doc or xls extension", filename: "survey.bin", contents: append(append([]byte{}, compoundFileSignature...), make([]byte, 512)...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := detectEvidenceUploadFile(memoryMultipartFile{bytes.NewReader(tt.contents)}, int64(len(tt.contents)), tt.filename)
			if err == nil {
				t.Fatal("detectEvidenceUploadFile() error = nil, want validation error")
			}
			var statusErr StatusError
			if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
				t.Fatalf("detectEvidenceUploadFile() error = %#v, want VALIDATION_ERROR", err)
			}
		})
	}
}

func TestNewEvidenceAlwaysUsesInternalVisibility(t *testing.T) {
	t.Parallel()

	for _, requestedVisibility := range []string{"", "internal", "project", "client", "unsupported"} {
		t.Run(requestedVisibility, func(t *testing.T) {
			normalized, err := validateWorkItemEvidenceRequest(WorkItemEvidenceRequest{
				EvidenceType: "document",
				Name:         "Biên bản",
				FileURL:      "https://example.com/evidence.pdf",
				Visibility:   requestedVisibility,
			})
			if err != nil {
				t.Fatalf("validateWorkItemEvidenceRequest() error = %v", err)
			}
			if normalized.Visibility != "internal" {
				t.Fatalf("visibility = %q, want internal", normalized.Visibility)
			}
		})
	}
}

func openXMLTestFile(t *testing.T, mainPart string, includeMacro bool) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entries := []string{"[Content_Types].xml", mainPart}
	if includeMacro {
		if len(mainPart) >= len("word/") && mainPart[:len("word/")] == "word/" {
			entries = append(entries, "word/vbaProject.bin")
		} else {
			entries = append(entries, "xl/vbaProject.bin")
		}
	}
	for _, name := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte("test")); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close test zip: %v", err)
	}
	return buffer.Bytes()
}
