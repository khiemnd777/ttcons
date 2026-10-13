package core

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func bmgsTestCellPhoto(t *testing.T, width, height int, shade color.RGBA) BMGSImage {
	t.Helper()
	pic := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pic.SetRGBA(x, y, shade)
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, pic); err != nil {
		t.Fatal(err)
	}
	im, err := prepareBMGSImage(raw.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	im.EvidenceID = "prototype-evidence"
	im.SourceRevisionKey = "evidence:prototype-evidence:0:prototype-evidence"
	im.Filename = "Ảnh cùng tên.png"
	return im
}

func bmgsTestZipParts(t *testing.T, raw []byte) map[string][]byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string][]byte{}
	for _, file := range z.File {
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		parts[file.Name] = data
	}
	return parts
}

func bmgsTestImagesForRows(t *testing.T, rows []Record) []BMGSImage {
	t.Helper()
	images := []BMGSImage{}
	for i, row := range rows {
		im := bmgsTestCellPhoto(t, 80, 40, color.RGBA{uint8(80 + i), 130, 180, 255})
		im.EvidenceID = fmt.Sprintf("image-%d", i)
		im.SourceRevisionKey = fmt.Sprintf("evidence:%s:0:%s", im.EvidenceID, im.EvidenceID)
		row["image_key"] = im.SourceRevisionKey
		images = append(images, im)
	}
	return images
}

// Inspect the actual sheet -> drawing -> image relationship chain and decoded
// extents, rather than accepting imports, filenames or anchor counts alone.
func bmgsAssertCompatibleImages(t *testing.T, raw []byte, images []bmgsCellImage) {
	t.Helper()
	parts := bmgsTestZipParts(t, raw)
	for name, data := range parts {
		if name == "xl/metadata.xml" || strings.HasPrefix(name, "xl/richData/") {
			t.Fatal("rich image part retained", name)
		}
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			var value any
			if err := xml.Unmarshal(data, &value); err != nil {
				t.Fatal(name, err)
			}
			if bytes.Contains(data, []byte("#VALUE!")) {
				t.Fatal("cached image error", name)
			}
		}
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type rel struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
		Mode   string `xml:"TargetMode,attr"`
	}
	relationships := func(part string) map[string]string {
		var v struct {
			Items []rel `xml:"Relationship"`
		}
		if err := xml.Unmarshal(parts[part], &v); err != nil {
			t.Fatal(part, err)
		}
		out := map[string]string{}
		for _, r := range v.Items {
			if r.Mode == "External" {
				t.Fatal("external image dependency")
			}
			out[r.ID] = r.Target
		}
		return out
	}
	resolve := func(base, target string) string {
		if strings.HasPrefix(target, "/") {
			return strings.TrimPrefix(target, "/")
		}
		return path.Clean(path.Join(path.Dir(base), target))
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			ID   string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(parts["xl/workbook.xml"], &wb); err != nil {
		t.Fatal(err)
	}
	wbRels := relationships("xl/_rels/workbook.xml.rels")
	expected := map[string]BMGSImage{}
	for _, im := range images {
		expected[im.Sheet+":"+im.Cell] = im.Image
	}
	for _, s := range wb.Sheets {
		hasExpected := false
		for _, im := range images {
			if im.Sheet == s.Name {
				hasExpected = true
				break
			}
		}
		if !hasExpected {
			continue
		}
		sheetPart := resolve("xl/workbook.xml", wbRels[s.ID])
		var ws struct {
			Drawing struct {
				ID string `xml:"id,attr"`
			} `xml:"drawing"`
		}
		if err := xml.Unmarshal(parts[sheetPart], &ws); err != nil {
			t.Fatal(err)
		}
		if ws.Drawing.ID == "" {
			continue
		}
		drawingPart := resolve(sheetPart, relationships(path.Join(path.Dir(sheetPart), "_rels", path.Base(sheetPart)+".rels"))[ws.Drawing.ID])
		var drawing struct {
			Two     []struct{} `xml:"twoCellAnchor"`
			Anchors []struct {
				From struct {
					Col    int `xml:"col"`
					Row    int `xml:"row"`
					ColOff int `xml:"colOff"`
					RowOff int `xml:"rowOff"`
				} `xml:"from"`
				Ext     bmgsDrawingExtent `xml:"ext"`
				Picture struct {
					Props bmgsDrawingProperties `xml:"nvPicPr>cNvPr"`
					Blip  struct {
						ID    string `xml:"embed,attr"`
						State string `xml:"cstate,attr"`
					} `xml:"blipFill>blip"`
					Xfrm *struct{} `xml:"spPr>xfrm"`
				} `xml:"pic"`
			} `xml:"oneCellAnchor"`
		}
		if err := xml.Unmarshal(parts[drawingPart], &drawing); err != nil {
			t.Fatal(err)
		}
		if len(drawing.Two) > 0 {
			t.Fatal("two-cell anchors do not match approved probe")
		}
		rels := relationships(path.Join(path.Dir(drawingPart), "_rels", path.Base(drawingPart)+".rels"))
		for _, a := range drawing.Anchors {
			cell, err := excelize.CoordinatesToCellName(a.From.Col+1, a.From.Row+1)
			if err != nil {
				t.Fatal(err)
			}
			key := s.Name + ":" + cell
			im, ok := expected[key]
			if !ok {
				t.Fatal("wrong or duplicate image row", key)
			}
			delete(expected, key)
			if a.From.ColOff != 0 || a.From.RowOff != 0 || a.Picture.Xfrm != nil || a.Picture.Blip.State != "print" {
				t.Fatal("probe shape changed", key)
			}
			if !strings.Contains(a.Picture.Props.Description, im.Filename) || !strings.Contains(a.Picture.Props.Description, im.SourceRevisionKey) || !strings.Contains(a.Picture.Props.Description, im.EvidenceID) {
				t.Fatal("image provenance lost", key)
			}
			media := resolve(drawingPart, rels[a.Picture.Blip.ID])
			if !bytes.Equal(parts[media], im.Bytes) {
				t.Fatal("wrong exact image bytes", key, media)
			}
			height, err := f.GetRowHeight(s.Name, a.From.Row+1)
			if err != nil {
				t.Fatal(err)
			}
			width, err := f.GetColWidth(s.Name, "F")
			if err != nil {
				t.Fatal(err)
			}
			if a.Ext.CX <= 0 || a.Ext.CY <= 0 || float64(a.Ext.CX) > (bmgsColumnPixels(width)-5)*9525 || float64(a.Ext.CY) > (height/.75-6)*9525 {
				t.Fatal("image escapes cell", key, a.Ext, width, height)
			}
			// The two independent EMU floors can differ by at most one unit each.
			if math.Abs(float64(a.Ext.CX)*float64(im.Height)-float64(a.Ext.CY)*float64(im.Width)) > float64(im.Width+im.Height) {
				t.Fatal("image aspect ratio changed", key)
			}
			value, err := f.GetCellValue(s.Name, cell)
			if err != nil || value != "" {
				t.Fatal("image cell stores error or filename", key, value, err)
			}
			pics, err := f.GetPictures(s.Name, cell)
			if err != nil || len(pics) != 1 || !bytes.Equal(pics[0].File, im.Bytes) {
				t.Fatal("standard reader cannot resolve embedded drawing", key, err, len(pics))
			}
		}
	}
	if len(expected) != 0 {
		t.Fatal("missing image anchors", expected)
	}
}

func bmgsSaveCellQA(t *testing.T, name string, raw []byte) {
	t.Helper()
	if dir := os.Getenv("BMGS_CELL_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBMGSCellImagesNativeRowsAndFit(t *testing.T) {
	rows, images := []Record{}, []BMGSImage{}
	for i, size := range [][2]int{{240, 400}, {400, 240}, {1, 1280}, {1280, 1}} {
		id := fmt.Sprintf("evidence-%d", i)
		ref := BMGSSourceRef{Kind: "evidence", ID: id, RevisionNo: i}
		sheet, err := nativeBMGSProjectSheet("BM-GS-07", ref, Record{"id": id, "name": "Ảnh <trùng>&.png", "caption": fmt.Sprintf("Nội dung đúng dòng %d", i+1), "payload_json": Record{"capture_metadata": Record{"location": fmt.Sprintf("Vị trí %d", i+1), "photo_phase": "before"}}})
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, sheet.Rows[0])
		im := bmgsTestCellPhoto(t, size[0], size[1], color.RGBA{uint8(40 + i*50), 140, 190, 255})
		im.EvidenceID, im.SourceRevisionKey, im.Filename = id, nativeBMGSImageIdentity(ref, id), "Ảnh <trùng>&.png"
		images = append(images, im)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-07", Sheets: []BMGSSheet{{Rows: rows}}, Images: []BMGSImage{images[3], images[1], images[0], images[2]}})
	if err != nil {
		t.Fatal(err)
	}
	bindings := []bmgsCellImage{}
	for i, im := range images {
		bindings = append(bindings, bmgsCellImage{Sheet: "BM-GS-07", Cell: fmt.Sprintf("F%d", i+8), Image: im})
	}
	bmgsAssertCompatibleImages(t, out, bindings)
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if len(f.GetSheetList()) != 1 {
		t.Fatal("image appendix retained")
	}
	for i := range images {
		value, _ := f.GetCellValue("BM-GS-07", fmt.Sprintf("B%d", i+8))
		if value != fmt.Sprintf("Vị trí %d", i+1) {
			t.Fatal("source row moved", value)
		}
	}
	portrait, _ := f.GetRowHeight("BM-GS-07", 8)
	landscape, _ := f.GetRowHeight("BM-GS-07", 9)
	if portrait <= landscape {
		t.Fatal("aspect ratio ignored in row sizing")
	}
	template, err := excelize.OpenReader(bytes.NewReader(bmgsTemplateWorkbook))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := template.GetCellValue("BM-GS-07", "F7")
	template.Close()
	header, _ := f.GetCellValue("BM-GS-07", "F7")
	if header != want || header == "" {
		t.Fatal("client header changed")
	}
	if _, err = bmgsWriteCellImages(out, bindings); err == nil {
		t.Fatal("existing drawings silently overwritten")
	}
	bmgsSaveCellQA(t, "BM07-compatible-multiple.xlsx", out)
}

func TestBMGSCellImagesDynamicDimensionsMultipleSheets(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Ảnh đổi tên")
	f.NewSheet("Second register")
	bindings := []bmgsCellImage{}
	for i, tc := range []struct {
		sheet         string
		width, height float64
		row           int
		size          [2]int
	}{
		{"Ảnh đổi tên", 9.25, 73, 8, [2]int{600, 200}},
		{"Ảnh đổi tên", 9.25, 180, 9, [2]int{200, 600}},
		{"Second register", 42, 302, 8, [2]int{1280, 1}},
		{"Second register", 42, 60, 9, [2]int{1, 1280}},
	} {
		cell := fmt.Sprintf("F%d", tc.row)
		f.SetColWidth(tc.sheet, "F", "F", tc.width)
		f.SetRowHeight(tc.sheet, tc.row, tc.height)
		f.SetCellStr(tc.sheet, cell, "")
		im := bmgsTestCellPhoto(t, tc.size[0], tc.size[1], color.RGBA{uint8(i * 40), 70, 140, 255})
		im.EvidenceID = fmt.Sprintf("dynamic-%d", i)
		im.SourceRevisionKey = "evidence:" + im.EvidenceID + ":0:" + im.EvidenceID
		bindings = append(bindings, bmgsCellImage{Sheet: tc.sheet, Cell: cell, Image: im})
	}
	f.SetCellStr("Second register", "D8", "Preserved neighbouring content")
	buffer, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	out, err := bmgsWriteCellImages(buffer.Bytes(), bindings)
	if err != nil {
		t.Fatal(err)
	}
	bmgsAssertCompatibleImages(t, out, bindings)
	reopened, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	value, _ := reopened.GetCellValue("Second register", "D8")
	if value != "Preserved neighbouring content" {
		t.Fatal("neighbouring text changed")
	}
	height, _ := reopened.GetRowHeight("Second register", 8)
	if height != 302 {
		t.Fatal("final text-driven row height changed")
	}
	bmgsSaveCellQA(t, "BM07-compatible-dynamic-sheets.xlsx", out)
}

func TestBMGSCellImagesGroupedSourcesRenamedSheetAndLongText(t *testing.T) {
	sources, images := []BMGSSheet{}, []BMGSImage{}
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("source-%d", i)
		ref := BMGSSourceRef{Kind: "evidence", ID: id, RevisionNo: i}
		s, err := nativeBMGSProjectSheet("BM-GS-07", ref, Record{"id": id, "name": "trùng.png", "caption": strings.Repeat("Nội dung kỹ thuật đã ghi nhận\n", 12), "payload_json": Record{"capture_metadata": Record{"location": id}}})
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, s)
		im := bmgsTestCellPhoto(t, 400, 120, color.RGBA{uint8(i * 40), 100, 160, 255})
		im.EvidenceID, im.SourceRevisionKey = id, nativeBMGSImageIdentity(ref, id)
		images = append(images, im)
	}
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-07", Sheets: sources, Images: images})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if len(f.GetSheetList()) != 1 {
		t.Fatal("sources not grouped")
	}
	bindings := []bmgsCellImage{}
	for i, im := range images {
		row := i + 8
		height, _ := f.GetRowHeight("BM-GS-07", row)
		styleID, _ := f.GetCellStyle("BM-GS-07", fmt.Sprintf("D%d", row))
		style, err := f.GetStyle(styleID)
		if err != nil || style.Font == nil || height < 12*style.Font.Size*1.35+6 {
			t.Fatal("wrapped text clipped", height, err)
		}
		bindings = append(bindings, bmgsCellImage{Sheet: "BM-GS-07", Cell: fmt.Sprintf("F%d", row), Image: im})
	}
	bmgsAssertCompatibleImages(t, out, bindings)
	// Binding also derives height from arbitrary current column width.
	original := excelize.NewFile()
	defer original.Close()
	original.SetSheetName("Sheet1", "Renamed")
	original.SetColWidth("Renamed", "F", "F", 41)
	original.SetRowHeight("Renamed", 8, 302)
	bound, err := bmgsBindCellImages(original, "Renamed", []Record{{"image_key": images[0].SourceRevisionKey}}, []BMGSImage{images[0]})
	if err != nil || len(bound) != 1 || bound[0].Sheet != "Renamed" {
		t.Fatal(err)
	}
	height, _ := original.GetRowHeight("Renamed", 8)
	if height != 302 {
		t.Fatal("long text row height shrunk")
	}
}

func TestBMGSCellImagesMissingConflictingIdentity(t *testing.T) {
	im := bmgsTestCellPhoto(t, 40, 30, color.RGBA{90, 150, 170, 255})
	base := Record{"image_key": im.SourceRevisionKey}
	wrongID := im
	wrongID.EvidenceID = "conflicting-evidence"
	wrongBytes := im
	wrongBytes.Bytes = []byte("invalid image bytes")
	for _, tc := range []struct {
		name   string
		rows   []Record
		images []BMGSImage
	}{
		{"missing row key", []Record{{}}, []BMGSImage{im}}, {"missing image", []Record{base}, nil}, {"orphan image", nil, []BMGSImage{im}},
		{"duplicate image key", []Record{base}, []BMGSImage{im, im}}, {"duplicate row key", []Record{base, base}, []BMGSImage{im}},
		{"key conflicts with evidence", []Record{base}, []BMGSImage{wrongID}}, {"corrupt optimized bytes", []Record{base}, []BMGSImage{wrongBytes}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-07", Sheets: []BMGSSheet{{Rows: tc.rows}}, Images: tc.images}); err == nil {
				t.Fatal("ambiguous/incomplete image log succeeded")
			}
		})
	}
}

func TestBMGSCellImagesDynamicRowsRetain51(t *testing.T) {
	rows := []Record{}
	for i := 0; i < 51; i++ {
		rows = append(rows, Record{"location": fmt.Sprintf("Source row %d", i+1), "technical_content": "Associated image", "filename": "Duplicate filename.png"})
	}
	images := bmgsTestImagesForRows(t, rows)
	out, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: "BM-GS-07", Sheets: []BMGSSheet{{Rows: rows}}, Images: images})
	if err != nil {
		t.Fatal(err)
	}
	bindings := []bmgsCellImage{}
	for i, im := range images {
		bindings = append(bindings, bmgsCellImage{Sheet: "BM-GS-07", Cell: fmt.Sprintf("F%d", i+8), Image: im})
	}
	bmgsAssertCompatibleImages(t, out, bindings)
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	last, _ := f.GetCellValue("BM-GS-07", "B58")
	if last != "Source row 51" {
		t.Fatal("last row truncated")
	}
	found := false
	for _, name := range f.GetDefinedName() {
		if name.Name == "_xlnm.Print_Area" && name.Scope == "BM-GS-07" {
			found = true
			if name.RefersTo != "'BM-GS-07'!$A$1:$G$66" {
				t.Fatal("print area clipped", name.RefersTo)
			}
		}
	}
	if !found {
		t.Fatal("print area missing")
	}
	bmgsSaveCellQA(t, "BM07-compatible-51-images.xlsx", out)
}

func TestBMGSCellImagesWriterRejectsInvalidTargetsAndLimits(t *testing.T) {
	im := bmgsTestCellPhoto(t, 40, 30, color.RGBA{90, 150, 170, 255})
	for _, tc := range []string{"wrong column", "header row", "unknown sheet", "nonblank target", "hidden column", "hidden row", "zero width", "small row", "wrong MIME", "empty identity", "duplicate binding", "duplicate identity", "too many images", "bad dimensions"} {
		t.Run(tc, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			f.SetCellStr("Sheet1", "F8", "")
			f.SetRowHeight("Sheet1", 8, 60)
			f.SetColWidth("Sheet1", "F", "F", 24)
			bindings := []bmgsCellImage{{Sheet: "Sheet1", Cell: "F8", Image: im}}
			switch tc {
			case "wrong column":
				bindings[0].Cell = "G8"
			case "header row":
				bindings[0].Cell = "F7"
			case "unknown sheet":
				bindings[0].Sheet = "Missing"
			case "nonblank target":
				f.SetCellStr("Sheet1", "F8", "do not erase")
			case "hidden column":
				f.SetColVisible("Sheet1", "F", false)
			case "hidden row":
				f.SetRowVisible("Sheet1", 8, false)
			case "zero width":
				f.SetColWidth("Sheet1", "F", "F", 0)
			case "small row":
				f.SetRowHeight("Sheet1", 8, 3)
			case "wrong MIME":
				bindings[0].Image.MIMEType = "image/jpeg"
			case "empty identity":
				bindings[0].Image.SourceRevisionKey = ""
			case "duplicate binding":
				second := bindings[0]
				second.Image.EvidenceID = "second"
				second.Image.SourceRevisionKey = "evidence:second:0:second"
				bindings = append(bindings, second)
			case "duplicate identity":
				second := bindings[0]
				second.Cell = "F9"
				bindings = append(bindings, second)
			case "too many images":
				bindings = make([]bmgsCellImage, bmgsMaxImages+1)
			case "bad dimensions":
				bindings[0].Image.Width++
			}
			raw, err := f.WriteToBuffer()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bmgsWriteCellImages(raw.Bytes(), bindings); err == nil {
				t.Fatal("invalid writer input accepted")
			}
		})
	}
	// A ZIP with repeated part names must fail before the library can hide it.
	var raw bytes.Buffer
	z := zip.NewWriter(&raw)
	for i := 0; i < 2; i++ {
		w, _ := z.Create("xl/workbook.xml")
		w.Write([]byte("<workbook/>"))
	}
	z.Close()
	if _, err := bmgsWriteCellImages(raw.Bytes(), []bmgsCellImage{{Sheet: "Sheet1", Cell: "F8", Image: im}}); err == nil {
		t.Fatal("duplicate ZIP parts accepted")
	}
	raw.Reset()
	z = zip.NewWriter(&raw)
	header := &zip.FileHeader{Name: "xl/bomb.xml", Method: zip.Deflate, UncompressedSize64: uint64(bmgsMaxWorkbookBytes + 1), CompressedSize64: 1}
	entry, err := z.CreateRaw(header)
	if err != nil {
		t.Fatal(err)
	}
	entry.Write([]byte{0})
	z.Close()
	if _, err := bmgsReadImagePackage(raw.Bytes()); err == nil {
		t.Fatal("oversized uncompressed part accepted")
	}
}

func TestBMGSCellImagesPreserveOtherSheetDrawingAndDetectSparseCollision(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Image register")
	f.NewSheet("Existing appendix")
	f.SetColWidth("Image register", "F", "F", 24)
	f.SetRowHeight("Image register", 8, 70)
	f.SetCellStr("Image register", "F8", "")
	im := bmgsTestCellPhoto(t, 80, 40, color.RGBA{20, 100, 160, 255})
	old := bmgsTestCellPhoto(t, 70, 30, color.RGBA{120, 80, 60, 255})
	if err := f.AddPictureFromBytes("Existing appendix", "A1", &excelize.Picture{Extension: ".png", File: old.Bytes}); err != nil {
		t.Fatal(err)
	}
	raw, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	binding := []bmgsCellImage{{Sheet: "Image register", Cell: "F8", Image: im}}
	out, err := bmgsWriteCellImages(raw.Bytes(), binding)
	if err != nil {
		t.Fatal(err)
	}
	bmgsAssertCompatibleImages(t, out, binding)
	reopened, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	pics, err := reopened.GetPictures("Existing appendix", "A1")
	if err != nil || len(pics) != 1 || !bytes.Equal(pics[0].File, old.Bytes) {
		t.Fatal("other sheet image changed", err)
	}
	// Two existing media IDs numbered 1 and 3 expose Excelize's count-based
	// allocator. A new unique image would select image3: detect that collision.
	parts := bmgsTestZipParts(t, raw.Bytes())
	parts["xl/media/image3.png"] = old.Bytes
	var sparse bytes.Buffer
	z := zip.NewWriter(&sparse)
	for name, data := range parts {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	z.Close()
	if _, err := bmgsWriteCellImages(sparse.Bytes(), binding); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatal("sparse media collision was not rejected", err)
	}
	// A duplicated relationship must fail before library maps hide the first.
	parts = bmgsTestZipParts(t, raw.Bytes())
	rels := parts["xl/_rels/workbook.xml.rels"]
	var parsed struct {
		Items []struct {
			ID string `xml:"Id,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.Unmarshal(rels, &parsed); err != nil {
		t.Fatal(err)
	}
	duplicate := []byte(fmt.Sprintf("<Relationship Id=%q Type=\"test\" Target=\"test\"/>", parsed.Items[0].ID))
	parts["xl/_rels/workbook.xml.rels"] = bytes.Replace(rels, []byte("</Relationships>"), append(duplicate, []byte("</Relationships>")...), 1)
	sparse.Reset()
	z = zip.NewWriter(&sparse)
	for name, data := range parts {
		w, _ := z.Create(name)
		w.Write(data)
	}
	z.Close()
	if _, err := bmgsWriteCellImages(sparse.Bytes(), binding); err == nil || !strings.Contains(err.Error(), "duplicate relationship") {
		t.Fatal("relationship collision accepted", err)
	}
}
