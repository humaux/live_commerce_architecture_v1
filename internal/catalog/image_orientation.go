package catalog

// JPEG's EXIF orientation affects browser originals but not Go Decode. Read only bounded IFD0 orientation;
// no URLs, nested metadata traversal, writes or source metadata are copied into a derivative.
import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
)

func jpegOrientation(data []byte) uint16 {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for pos := 2; pos+4 <= len(data); {
		if data[pos] != 0xff {
			return 1
		}
		marker := data[pos+1]
		pos += 2
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if size < 2 || size > len(data)-pos {
			return 1
		}
		segment := data[pos+2 : pos+size]
		pos += size
		if marker != 0xe1 || !bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			continue
		}
		b := segment[6:]
		if len(b) < 8 {
			return 1
		}
		var order binary.ByteOrder
		switch string(b[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(b[2:4]) != 42 {
			return 1
		}
		offset := int64(order.Uint32(b[4:8]))
		if offset < 8 || offset > int64(len(b)-2) {
			return 1
		}
		count := int(order.Uint16(b[offset : offset+2]))
		start := int(offset) + 2
		for n := 0; n < count && n < (len(b)-start)/12; n++ {
			e := b[start+n*12 : start+(n+1)*12]
			if order.Uint16(e[:2]) == 0x112 && order.Uint16(e[2:4]) == 3 && order.Uint32(e[4:8]) == 1 {
				orientation := order.Uint16(e[8:10])
				if orientation >= 1 && orientation <= 8 {
					return orientation
				}
				return 1
			}
		}
		return 1
	}
	return 1
}

// orientPhoto maps the eight EXIF transforms without allocating a second full decoded bitmap.
func orientPhoto(src image.Image, orientation uint16) image.Image {
	if orientation < 2 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if orientation >= 5 {
		w, h = h, w
	}
	return orientedPhoto{Image: src, orientation: orientation, bounds: image.Rect(0, 0, w, h)}
}

type orientedPhoto struct {
	image.Image
	orientation uint16
	bounds      image.Rectangle
}

func (p orientedPhoto) Bounds() image.Rectangle { return p.bounds }
func (p orientedPhoto) At(x, y int) color.Color {
	w, h := p.Image.Bounds().Dx(), p.Image.Bounds().Dy()
	switch p.orientation {
	case 2:
		x = w - 1 - x
	case 3:
		x, y = w-1-x, h-1-y
	case 4:
		y = h - 1 - y
	case 5:
		x, y = y, x
	case 6:
		x, y = y, h-1-x
	case 7:
		x, y = w-1-y, h-1-x
	case 8:
		x, y = w-1-y, x
	}
	return p.Image.At(x+p.Image.Bounds().Min.X, y+p.Image.Bounds().Min.Y)
}
