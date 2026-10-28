// Copyright (c) 2026 MyGo contributors
// SPDX-License-Identifier: MIT
// EXIF tag parsing is adapted from third_party/mygo/ui/bitmap.go; the full
// license is retained in third_party/mygo/LICENSE and THIRD_PARTY_NOTICES.
package app

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
)

// As Flutter's ResizeImagePolicy.fit does, retain at most 192 pixels on the
// longest side. Orient JPEGs before scaling without allocating a rotated copy.
func iconBitmap(data []byte) (*ui.Bitmap, int, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	if orientation := exifOrientation(data); orientation > 1 {
		src = orientedIcon{src, orientation}
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > 192 || height > 192 {
		if width >= height {
			height = max(1, (height*192+width/2)/width)
			width = 192
		} else {
			width = max(1, (width*192+height/2)/height)
			height = 192
		}
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Src, nil)
	return ui.NewBitmap(dst), width * height * 4, nil
}

type orientedIcon struct {
	image.Image
	orientation int
}

func (im orientedIcon) Bounds() image.Rectangle {
	b := im.Image.Bounds()
	w, h := b.Dx(), b.Dy()
	if im.orientation >= 5 {
		w, h = h, w
	}
	return image.Rect(0, 0, w, h)
}
func (im orientedIcon) At(x, y int) color.Color {
	b := im.Image.Bounds()
	w, h := b.Dx(), b.Dy()
	var sx, sy int
	switch im.orientation {
	case 2:
		sx, sy = w-1-x, y
	case 3:
		sx, sy = w-1-x, h-1-y
	case 4:
		sx, sy = x, h-1-y
	case 5:
		sx, sy = y, x
	case 6:
		sx, sy = y, h-1-x
	case 7:
		sx, sy = w-1-y, h-1-x
	case 8:
		sx, sy = w-1-y, x
	default:
		sx, sy = x, y
	}
	return im.Image.At(sx+b.Min.X, sy+b.Min.Y)
}

// exifOrientation returns the orientation of a JPEG's EXIF data, 1 to 8,
// or 0 without one.
func exifOrientation(jpg []byte) int {
	if len(jpg) < 4 || jpg[0] != 0xff || jpg[1] != 0xd8 {
		return 0
	}
	for i := 2; i+4 <= len(jpg); {
		if jpg[i] != 0xff {
			return 0
		}
		marker, size := jpg[i+1], int(binary.BigEndian.Uint16(jpg[i+2:]))
		if marker == 0xda || size < 2 || i+2+size > len(jpg) { // the image data starts
			return 0
		}
		seg := jpg[i+4 : i+2+size]
		if marker == 0xe1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 0
}

// tiffOrientation reads the Orientation tag of the first IFD of TIFF data.
func tiffOrientation(t []byte) int {
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 0
	}
	n := int(bo.Uint16(t[ifd:]))
	for e := ifd + 2; e+12 <= len(t) && n > 0; e, n = e+12, n-1 {
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 { // Orientation, a SHORT
			if o := int(bo.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 0
		}
	}
	return 0
}
