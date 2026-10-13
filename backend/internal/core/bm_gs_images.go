package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const bmgsMaxInputImageBytes = 20 << 20
const bmgsMaxImagePixels = 40_000_000
const bmgsMaxOutputImageBytes = 5 << 20
const bmgsMaxImages = 250
const bmgsMaxWorkbookImageBytes = 100 << 20
const bmgsMaxWorkbookBytes = 150 << 20

// prepareBMGSImage validates dimensions before decoding and strips source metadata.
func prepareBMGSImage(raw []byte, mimeType string) (BMGSImage, error) {
	var result BMGSImage
	if len(raw) == 0 || len(raw) > bmgsMaxInputImageBytes {
		return result, fmt.Errorf("image input must be 1..%d bytes", bmgsMaxInputImageBytes)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return result, fmt.Errorf("invalid image: %w", err)
	}
	if format != "jpeg" && format != "png" && format != "webp" && format != "gif" {
		return result, fmt.Errorf("unsupported image format %q", format)
	}
	expected := "image/" + format
	if format == "jpeg" {
		expected = "image/jpeg"
	}
	if strings.TrimSpace(strings.Split(mimeType, ";")[0]) != expected {
		return result, fmt.Errorf("image MIME does not match %s", format)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > bmgsMaxImagePixels {
		return result, fmt.Errorf("image exceeds pixel limit")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return result, fmt.Errorf("decode image: %w", err)
	}
	src = bmgsOrient(src, bmgsContainerOrientation(raw, format))
	if format == "gif" {
		// Excel embeds a static image. Decode only the first frame (not all
		// animation frames) and retain transparent/detail pixels as PNG.
		canvas := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
		draw.Draw(canvas, src.Bounds(), src, src.Bounds().Min, draw.Src)
		src = canvas
		format = "png"
		expected = "image/png"
		result.DerivativeNote = "GIF: khung hình đầu tiên"
	}
	if format == "webp" {
		// Preserve transparent WebP pixels; opaque photographs use JPEG80.
		format = "jpeg"
		if opaque, ok := src.(interface{ Opaque() bool }); !ok || !opaque.Opaque() {
			format = "png"
		}
		expected = "image/" + format
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w > 1280 || h > 1280 {
		if w >= h {
			h = max(1, h*1280/w)
			w = 1280
		} else {
			w = max(1, w*1280/h)
			h = 1280
		}
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		src = dst
	}
	var out bytes.Buffer
	if format == "png" {
		err = png.Encode(&out, src)
	} else {
		err = jpeg.Encode(&out, src, &jpeg.Options{Quality: 80})
	}
	if err != nil {
		return result, fmt.Errorf("encode image: %w", err)
	}
	if out.Len() > bmgsMaxOutputImageBytes {
		return result, fmt.Errorf("optimized image exceeds output limit")
	}
	result.Bytes = out.Bytes()
	result.Width = w
	result.Height = h
	result.MIMEType = expected
	return result, nil
}

// EXIF is optional. Invalid EXIF never affects a valid JPEG's pixels or causes a panic.
func bmgsEXIFOrientation(raw []byte) int {
	for p := 2; p+4 <= len(raw); {
		if raw[p] != 0xff {
			break
		}
		marker := raw[p+1]
		if marker == 0xda || marker == 0xd9 {
			break
		}
		n := int(binary.BigEndian.Uint16(raw[p+2 : p+4]))
		if n < 2 || p+2+n > len(raw) {
			break
		}
		seg := raw[p+4 : p+2+n]
		p += 2 + n
		if marker != 0xe1 || len(seg) < 14 || string(seg[:6]) != "Exif\x00\x00" {
			continue
		}
		return bmgsTIFFOrientation(seg[6:])
	}
	return 1
}

func bmgsContainerOrientation(raw []byte, format string) int {
	if format == "jpeg" {
		return bmgsEXIFOrientation(raw)
	}
	if format == "png" {
		for p := 8; p+12 <= len(raw); {
			n := int(binary.BigEndian.Uint32(raw[p : p+4]))
			if n < 0 || n > len(raw)-p-12 {
				break
			}
			if string(raw[p+4:p+8]) == "eXIf" {
				return bmgsTIFFOrientation(raw[p+8 : p+8+n])
			}
			p += n + 12
		}
	}
	if format == "webp" {
		for p := 12; p+8 <= len(raw); {
			n := int(binary.LittleEndian.Uint32(raw[p+4 : p+8]))
			if n < 0 || n > len(raw)-p-8 {
				break
			}
			if string(raw[p:p+4]) == "EXIF" {
				t := raw[p+8 : p+8+n]
				if len(t) >= 6 && string(t[:6]) == "Exif\x00\x00" {
					t = t[6:]
				}
				return bmgsTIFFOrientation(t)
			}
			p += 8 + n + n%2
		}
	}
	return 1
}

func bmgsTIFFOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	if string(t[:2]) == "II" {
		order = binary.LittleEndian
	} else if string(t[:2]) == "MM" {
		order = binary.BigEndian
	} else {
		return 1
	}
	if order.Uint16(t[2:4]) != 42 {
		return 1
	}
	off := int(order.Uint32(t[4:8]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[off : off+2]))
	for i := 0; i < count; i++ {
		q := off + 2 + i*12
		if q+12 > len(t) {
			break
		}
		if order.Uint16(t[q:q+2]) == 0x112 && order.Uint16(t[q+2:q+4]) == 3 && order.Uint32(t[q+4:q+8]) == 1 {
			v := int(order.Uint16(t[q+8 : q+10]))
			if v >= 1 && v <= 8 {
				return v
			}
		}
	}
	return 1
}

func bmgsOrient(src image.Image, orientation int) image.Image {
	if orientation == 1 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
