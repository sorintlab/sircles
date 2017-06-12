package util

import (
	"bytes"
	"hash"
	"image"
	"image/color"

	"github.com/dchest/siphash"
	"github.com/disintegration/imaging"
	"github.com/pkg/errors"
)

const AvatarSize = 420

func RandomAvatar(s string) image.Image {
	key := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	id := new7x7(key)
	return id.render([]byte(s))
}

func RandomAvatarPNG(s string) ([]byte, error) {
	a := RandomAvatar(s)

	buf := &bytes.Buffer{}
	if err := imaging.Encode(buf, a, imaging.PNG); err != nil {
		return nil, errors.Wrap(err, "failed to encode avatar")
	}

	return buf.Bytes(), nil
}

func CropResizeAvatar(avatar []byte, cropX, cropY, cropSize int) ([]byte, error) {
	m, _, err := image.Decode(bytes.NewReader(avatar))
	if err != nil {
		return nil, err
	}

	m = imaging.Crop(m, image.Rect(cropX, cropY, cropX+cropSize, cropY+cropSize))

	m = imaging.Resize(m, AvatarSize, AvatarSize, imaging.Lanczos)
	buf := &bytes.Buffer{}
	if err := imaging.Encode(buf, m, imaging.PNG); err != nil {
		return nil, errors.Wrap(err, "failed to encode avatar")
	}

	return buf.Bytes(), nil
}

// Code adapted from github.com/dgryski/go-identicon
type identicon struct {
	key    []byte
	sqSize int
	rows   int
	cols   int
	h      hash.Hash64
}

const xborder = 35
const yborder = 35
const maxX = 420
const maxY = 420

func new7x7(key []byte) *identicon {
	return &identicon{
		sqSize: 50,
		rows:   7,
		cols:   7,
		h:      siphash.New(key),
	}
}

func (icon *identicon) render(data []byte) image.Image {

	icon.h.Reset()
	icon.h.Write(data)
	h := icon.h.Sum64()

	nrgba := color.NRGBA{
		R: uint8(h),
		G: uint8(h >> 8),
		B: uint8(h >> 16),
		A: 0xff,
	}
	h >>= 24

	img := image.NewPaletted(image.Rect(0, 0, maxX, maxY), color.Palette{color.NRGBA{0xf0, 0xf0, 0xf0, 0xff}, nrgba})

	sqx := 0
	sqy := 0

	pixels := make([]byte, icon.sqSize)
	for i := 0; i < icon.sqSize; i++ {
		pixels[i] = 1
	}

	for i := 0; i < icon.rows*(icon.cols+1)/2; i++ {

		if h&1 == 1 {

			for i := 0; i < icon.sqSize; i++ {
				x := xborder + sqx*icon.sqSize
				y := yborder + sqy*icon.sqSize + i
				offs := img.PixOffset(x, y)
				copy(img.Pix[offs:], pixels)

				x = xborder + (icon.cols-1-sqx)*icon.sqSize
				offs = img.PixOffset(x, y)
				copy(img.Pix[offs:], pixels)
			}
		}

		h >>= 1
		sqy++
		if sqy == icon.rows {
			sqy = 0
			sqx++
		}
	}

	return img
}
