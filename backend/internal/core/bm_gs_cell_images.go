package core

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

type bmgsCellImage struct {
	Sheet, Cell string
	Image       BMGSImage
}

const bmgsPackageRelNS = "http://schemas.openxmlformats.org/package/2006/relationships"

// Spreadsheet column widths use character units. This is the standard 96-dpi
// seven-pixel maximum digit width conversion, including five pixels of padding.
// The same conversion is used for row sizing and final drawing containment.
func bmgsColumnPixels(width float64) float64 {
	if width < 1 {
		return math.Floor(width*12 + .5)
	}
	return math.Floor(width*7+.5) + 5
}

func bmgsImageIdentity(im BMGSImage) error {
	if strings.TrimSpace(im.SourceRevisionKey) == "" || strings.TrimSpace(im.EvidenceID) == "" {
		return fmt.Errorf("image log requires exact source/evidence identity")
	}
	if !strings.HasSuffix(im.SourceRevisionKey, ":"+im.EvidenceID) {
		return fmt.Errorf("image source key conflicts with evidence identity")
	}
	return nil
}

func bmgsBindCellImages(f *excelize.File, sheet string, rows []Record, images []BMGSImage) ([]bmgsCellImage, error) {
	if len(images) > bmgsMaxImages {
		return nil, fmt.Errorf("image count exceeds %d", bmgsMaxImages)
	}
	width, err := f.GetColWidth(sheet, "F")
	if err != nil {
		return nil, err
	}
	availableWidth := bmgsColumnPixels(width) - 5
	if availableWidth <= 0 {
		return nil, fmt.Errorf("image column has no visible width")
	}
	byKey := map[string]BMGSImage{}
	for _, im := range images {
		if err := bmgsImageIdentity(im); err != nil {
			return nil, err
		}
		if _, exists := byKey[im.SourceRevisionKey]; exists {
			return nil, fmt.Errorf("conflicting image log identity %s", im.SourceRevisionKey)
		}
		byKey[im.SourceRevisionKey] = im
	}
	bindings := []bmgsCellImage{}
	used := map[string]bool{}
	for i, row := range rows {
		key := cleanText(row["image_key"])
		im, exists := byKey[key]
		if key == "" || !exists || used[key] {
			return nil, fmt.Errorf("image log row %d has missing or conflicting image identity", i+1)
		}
		used[key] = true
		if im.Width <= 0 || im.Height <= 0 {
			return nil, fmt.Errorf("invalid cell image dimensions")
		}
		rn := 8 + i
		imageHeight := math.Min(240, math.Max(54, (availableWidth*float64(im.Height)/float64(im.Width)+6)*.75))
		textHeight, err := f.GetRowHeight(sheet, rn)
		if err != nil {
			return nil, err
		}
		if err = f.SetRowHeight(sheet, rn, math.Max(textHeight, imageHeight)); err != nil {
			return nil, err
		}
		bindings = append(bindings, bmgsCellImage{Sheet: sheet, Cell: fmt.Sprintf("F%d", rn), Image: im})
	}
	if len(used) != len(byKey) {
		return nil, fmt.Errorf("image log contains an image without its exact row")
	}
	return bindings, nil
}

type bmgsDrawingMarker struct {
	Col    int `xml:"xdr:col"`
	ColOff int `xml:"xdr:colOff"`
	Row    int `xml:"xdr:row"`
	RowOff int `xml:"xdr:rowOff"`
}
type bmgsDrawingExtent struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}
type bmgsDrawingProperties struct {
	ID          int    `xml:"id,attr"`
	Name        string `xml:"name,attr"`
	Description string `xml:"descr,attr,omitempty"`
}
type bmgsDrawingPicture struct {
	NonVisual struct {
		Properties        bmgsDrawingProperties `xml:"xdr:cNvPr"`
		PictureProperties struct {
			RelativeResize string `xml:"preferRelativeResize,attr"`
		} `xml:"xdr:cNvPicPr"`
	} `xml:"xdr:nvPicPr"`
	Fill struct {
		Blip struct {
			State string `xml:"cstate,attr"`
			Embed string `xml:"r:embed,attr"`
		} `xml:"a:blip"`
		Stretch struct {
			Rect struct{} `xml:"a:fillRect"`
		} `xml:"a:stretch"`
	} `xml:"xdr:blipFill"`
	Shape struct {
		Geometry struct {
			Preset string   `xml:"prst,attr"`
			Values struct{} `xml:"a:avLst"`
		} `xml:"a:prstGeom"`
		NoFill struct{} `xml:"a:noFill"`
	} `xml:"xdr:spPr"`
}
type bmgsDrawingAnchor struct {
	From    bmgsDrawingMarker  `xml:"xdr:from"`
	Extent  bmgsDrawingExtent  `xml:"xdr:ext"`
	Picture bmgsDrawingPicture `xml:"xdr:pic"`
	Client  struct {
		Locked string `xml:"fLocksWithSheet,attr"`
	} `xml:"xdr:clientData"`
}
type bmgsDrawing struct {
	XMLName xml.Name            `xml:"xdr:wsDr"`
	XDR     string              `xml:"xmlns:xdr,attr"`
	A       string              `xml:"xmlns:a,attr"`
	R       string              `xml:"xmlns:r,attr"`
	Anchors []bmgsDrawingAnchor `xml:"xdr:oneCellAnchor"`
}

func bmgsReadImagePackage(raw []byte) (map[string][]byte, error) {
	if len(raw) > bmgsMaxWorkbookBytes {
		return nil, fmt.Errorf("workbook exceeds byte budget")
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	parts := map[string][]byte{}
	total := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if _, exists := parts[file.Name]; exists {
			return nil, fmt.Errorf("duplicate workbook part %s", file.Name)
		}
		if file.Name != path.Clean(file.Name) || strings.HasPrefix(file.Name, "/") || strings.HasPrefix(file.Name, "../") {
			return nil, fmt.Errorf("invalid workbook part path %s", file.Name)
		}
		if file.UncompressedSize64 > uint64(bmgsMaxWorkbookBytes-total) {
			return nil, fmt.Errorf("uncompressed workbook exceeds byte budget")
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(stream, int64(bmgsMaxWorkbookBytes-total+1)))
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		total += len(data)
		if total > bmgsMaxWorkbookBytes {
			return nil, fmt.Errorf("uncompressed workbook exceeds byte budget")
		}
		parts[file.Name] = data
	}
	return parts, nil
}

// Validate package identities before Excelize loads them: map-based readers
// otherwise hide duplicate relationship IDs and zero-width columns can read
// back as a default width in the pinned library.
func bmgsImageSheetParts(parts map[string][]byte) (map[string]string, error) {
	relationships := map[string]map[string]string{}
	for name, data := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		var rels struct {
			Items []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal(data, &rels); err != nil {
			return nil, err
		}
		ids := map[string]string{}
		for _, r := range rels.Items {
			if r.ID == "" {
				return nil, fmt.Errorf("missing relationship identity")
			}
			if _, exists := ids[r.ID]; exists {
				return nil, fmt.Errorf("duplicate relationship identity")
			}
			ids[r.ID] = r.Target
		}
		relationships[name] = ids
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(parts["xl/workbook.xml"], &wb); err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, s := range wb.Sheets {
		target := relationships["xl/_rels/workbook.xml.rels"][s.RID]
		if target == "" {
			return nil, fmt.Errorf("missing worksheet relationship")
		}
		target = path.Clean(path.Join("xl", target))
		if strings.HasPrefix(relationships["xl/_rels/workbook.xml.rels"][s.RID], "/") {
			target = strings.TrimPrefix(relationships["xl/_rels/workbook.xml.rels"][s.RID], "/")
		}
		if _, exists := result[s.Name]; exists {
			return nil, fmt.Errorf("duplicate worksheet name")
		}
		if _, exists := parts[target]; !exists {
			return nil, fmt.Errorf("missing worksheet part")
		}
		result[s.Name] = target
	}
	return result, nil
}

// Excelize owns worksheet references, content types and embedded media. Its
// oneCell positioning is supported, but its extra shape transforms differ from
// the Google-import probe. Only newly created drawing parts are normalized with
// typed XML to that tested shape. Excel still stores a drawing, not Place in Cell.
func bmgsWriteCellImages(raw []byte, images []bmgsCellImage) ([]byte, error) {
	if len(images) == 0 {
		return raw, nil
	}
	if len(images) > bmgsMaxImages {
		return nil, fmt.Errorf("image count exceeds %d", bmgsMaxImages)
	}
	original, err := bmgsReadImagePackage(raw)
	if err != nil {
		return nil, err
	}
	for name := range original {
		if name == "xl/metadata.xml" || strings.HasPrefix(name, "xl/richData/") {
			return nil, fmt.Errorf("rich image metadata is not supported by compatible writer")
		}
	}
	sheetParts, err := bmgsImageSheetParts(original)
	if err != nil {
		return nil, err
	}
	validatedSheets := map[string]map[string]int{}
	for _, binding := range images {
		part, exists := sheetParts[binding.Sheet]
		if !exists {
			return nil, fmt.Errorf("unknown cell image sheet")
		}
		cells, validated := validatedSheets[part]
		if !validated {
			var ws struct {
				Columns []struct {
					Min   int      `xml:"min,attr"`
					Max   int      `xml:"max,attr"`
					Width *float64 `xml:"width,attr"`
				} `xml:"cols>col"`
				Cells []struct {
					Ref string `xml:"r,attr"`
				} `xml:"sheetData>row>c"`
				Drawing *struct{} `xml:"drawing"`
			}
			if err := xml.Unmarshal(original[part], &ws); err != nil {
				return nil, err
			}
			if ws.Drawing != nil {
				return nil, fmt.Errorf("existing drawing cannot be overwritten")
			}
			for _, col := range ws.Columns {
				if col.Min <= 6 && col.Max >= 6 && col.Width != nil && *col.Width <= 0 {
					return nil, fmt.Errorf("image column has no visible width")
				}
			}
			cells = map[string]int{}
			for _, cell := range ws.Cells {
				cells[cell.Ref]++
			}
			validatedSheets[part] = cells
		}
		if cells[binding.Cell] != 1 {
			return nil, fmt.Errorf("cell image target %s must exist exactly once", binding.Cell)
		}
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	seen, identities := map[string]bool{}, map[string]bool{}
	total := 0
	expected := map[string]bmgsDrawingExtent{}
	for _, binding := range images {
		im := binding.Image
		if err := bmgsImageIdentity(im); err != nil {
			return nil, err
		}
		if identities[im.SourceRevisionKey] {
			return nil, fmt.Errorf("duplicate cell image identity")
		}
		identities[im.SourceRevisionKey] = true
		cfg, format, err := imageConfig(im.Bytes)
		if err != nil {
			return nil, err
		}
		if cfg.Width != im.Width || cfg.Height != im.Height || im.Width > 1280 || im.Height > 1280 || len(im.Bytes) > bmgsMaxOutputImageBytes || (format != "png" && format != "jpeg") || im.MIMEType != "image/"+format {
			return nil, fmt.Errorf("cell image must be preprocessed")
		}
		total += len(im.Bytes)
		if total > bmgsMaxWorkbookImageBytes {
			return nil, fmt.Errorf("cell images exceed byte budget")
		}
		col, row, err := excelize.CellNameToCoordinates(binding.Cell)
		if err != nil || col != 6 || row < 8 {
			return nil, fmt.Errorf("invalid image log cell %s", binding.Cell)
		}
		key := binding.Sheet + ":" + binding.Cell
		if seen[key] {
			return nil, fmt.Errorf("duplicate cell image binding")
		}
		seen[key] = true
		width, err := f.GetColWidth(binding.Sheet, "F")
		if err != nil {
			return nil, err
		}
		height, err := f.GetRowHeight(binding.Sheet, row)
		if err != nil {
			return nil, err
		}
		visible, err := f.GetColVisible(binding.Sheet, "F")
		if err != nil || !visible {
			return nil, fmt.Errorf("image column is hidden")
		}
		visible, err = f.GetRowVisible(binding.Sheet, row)
		if err != nil || !visible {
			return nil, fmt.Errorf("image row is hidden")
		}
		availableWidth, availableHeight := bmgsColumnPixels(width)-5, height/0.75-6
		if availableWidth <= 0 || availableHeight <= 0 {
			return nil, fmt.Errorf("image cell has no visible area")
		}
		value, err := f.GetCellValue(binding.Sheet, binding.Cell)
		if err != nil || value != "" {
			return nil, fmt.Errorf("image target cell must be blank")
		}
		scale := math.Min(availableWidth/float64(im.Width), availableHeight/float64(im.Height))
		// Round in EMUs rather than whole pixels so extreme ratios never collapse
		// to zero or stretch. Floor ensures extents remain within the final cell.
		ext := bmgsDrawingExtent{CX: int64(math.Floor(float64(im.Width) * scale * 9525)), CY: int64(math.Floor(float64(im.Height) * scale * 9525))}
		if ext.CX <= 0 || ext.CY <= 0 {
			return nil, fmt.Errorf("image extent is below drawing precision")
		}
		alt := bmgsJoin(im.Filename, bmgsJoin(im.EvidenceID, bmgsJoin(im.SourceRevisionKey, im.DerivativeNote)))
		expected[key] = ext
		extension := ".png"
		if format == "jpeg" {
			extension = ".jpg"
		}
		if err := f.AddPictureFromBytes(binding.Sheet, binding.Cell, &excelize.Picture{Extension: extension, File: im.Bytes, Format: &excelize.GraphicOptions{ScaleX: scale, ScaleY: scale, AltText: alt, LockAspectRatio: true, Positioning: "oneCell"}}); err != nil {
			return nil, err
		}
	}
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	parts, err := bmgsReadImagePackage(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	// The pinned library allocates media/drawing IDs by count. Fail closed on a
	// sparse-ID collision instead of silently replacing another sheet's image.
	for name, before := range original {
		if strings.HasPrefix(name, "xl/media/") || strings.HasPrefix(name, "xl/drawings/") {
			if !bytes.Equal(before, parts[name]) {
				return nil, fmt.Errorf("existing image part collision %s", name)
			}
		}
	}
	// Resolve each generated drawing through sheet relationships, including
	// renamed sheets and worksheets whose physical part numbers differ.
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(parts["xl/workbook.xml"], &workbook); err != nil {
		return nil, err
	}
	type relationship struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	readRelationships := func(part string) (map[string]string, error) {
		var rels struct {
			Items []relationship `xml:"Relationship"`
		}
		if err := xml.Unmarshal(parts[part], &rels); err != nil {
			return nil, err
		}
		targets := map[string]string{}
		for _, r := range rels.Items {
			if _, exists := targets[r.ID]; exists {
				return nil, fmt.Errorf("duplicate relationship identity")
			}
			targets[r.ID] = r.Target
		}
		return targets, nil
	}
	targets, err := readRelationships("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	normalized := 0
	for _, sheet := range workbook.Sheets {
		hasImages := false
		for _, binding := range images {
			if binding.Sheet == sheet.Name {
				hasImages = true
				break
			}
		}
		if !hasImages {
			continue
		}
		sheetPart := path.Clean(path.Join("xl", targets[sheet.RID]))
		if strings.HasPrefix(targets[sheet.RID], "/") {
			sheetPart = strings.TrimPrefix(targets[sheet.RID], "/")
		}
		var ws struct {
			Drawing struct {
				RID string `xml:"id,attr"`
			} `xml:"drawing"`
		}
		if err := xml.Unmarshal(parts[sheetPart], &ws); err != nil {
			return nil, err
		}
		sheetRels, err := readRelationships(path.Join(path.Dir(sheetPart), "_rels", path.Base(sheetPart)+".rels"))
		if err != nil {
			return nil, err
		}
		drawingPart := path.Clean(path.Join(path.Dir(sheetPart), sheetRels[ws.Drawing.RID]))
		if _, exists := original[drawingPart]; exists {
			return nil, fmt.Errorf("existing drawing cannot be overwritten")
		}
		// Decode library XML by namespace-independent local names; marshal an
		// explicit prefix vocabulary matching the validated Google probe.
		var input struct {
			Anchors []struct {
				From struct {
					Col    int `xml:"col"`
					Row    int `xml:"row"`
					ColOff int `xml:"colOff"`
					RowOff int `xml:"rowOff"`
				} `xml:"from"`
				Picture struct {
					Properties bmgsDrawingProperties `xml:"nvPicPr>cNvPr"`
					Blip       struct {
						Embed string `xml:"embed,attr"`
					} `xml:"blipFill>blip"`
				} `xml:"pic"`
			} `xml:"oneCellAnchor"`
		}
		if err := xml.Unmarshal(parts[drawingPart], &input); err != nil {
			return nil, err
		}
		drawing := bmgsDrawing{XDR: "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing", A: "http://schemas.openxmlformats.org/drawingml/2006/main", R: "http://schemas.openxmlformats.org/officeDocument/2006/relationships"}
		for _, in := range input.Anchors {
			cell, err := excelize.CoordinatesToCellName(in.From.Col+1, in.From.Row+1)
			if err != nil {
				return nil, err
			}
			key := sheet.Name + ":" + cell
			ext, exists := expected[key]
			if !exists || in.From.ColOff != 0 || in.From.RowOff != 0 || in.Picture.Blip.Embed == "" {
				return nil, fmt.Errorf("unexpected drawing anchor")
			}
			anchor := bmgsDrawingAnchor{From: bmgsDrawingMarker{Col: in.From.Col, Row: in.From.Row}, Extent: ext}
			anchor.Picture.NonVisual.Properties = in.Picture.Properties
			anchor.Picture.NonVisual.PictureProperties.RelativeResize = "0"
			anchor.Picture.Fill.Blip.State, anchor.Picture.Fill.Blip.Embed = "print", in.Picture.Blip.Embed
			anchor.Picture.Shape.Geometry.Preset = "rect"
			anchor.Client.Locked = "0"
			drawing.Anchors = append(drawing.Anchors, anchor)
			delete(expected, key)
			normalized++
		}
		data, err := xml.Marshal(drawing)
		if err != nil {
			return nil, err
		}
		parts[drawingPart] = data
	}
	if normalized != len(images) || len(expected) != 0 {
		return nil, fmt.Errorf("image drawing count mismatch")
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		stream, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := stream.Write(parts[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if out.Len() > bmgsMaxWorkbookBytes {
		return nil, fmt.Errorf("workbook exceeds byte budget")
	}
	return out.Bytes(), nil
}
