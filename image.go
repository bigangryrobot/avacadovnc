package vnc2video

import (
	"image"
	"image/color"
	"image/draw"
)

// Image is a custom in-memory image implementation whose At method returns color.RGBA values.
// It is functionally equivalent to the standard library's `image.RGBA` but is provided
// here for direct control over the pixel buffer and for historical compatibility within this codebase.
// For new code, using the standard `image.RGBA` is often preferred.
type Image struct {
	// Pix holds the image's pixels, in R, G, B, A order (non-premultiplied).
	// The pixel at (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4].
	Pix []uint8
	// Stride is the Pix stride (in bytes) between vertically adjacent pixels.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

// NewImage returns a new Image with the given bounds.
// The pixel buffer is allocated and initialized to all zeros (transparent black).
func NewImage(r image.Rectangle) *Image {
	w, h := r.Dx(), r.Dy()
	if w <= 0 || h <= 0 {
		return &Image{Rect: r}
	}
	buf := make([]uint8, 4*w*h)
	return &Image{Pix: buf, Stride: 4 * w, Rect: r}
}

// NewFromImage creates a new custom Image by drawing the contents of an existing
// standard `image.Image`. This is a utility function to convert from standard
// image types to this custom one.
func NewFromImage(src image.Image) *Image {
	b := src.Bounds()
	m := NewImage(b)
	draw.Draw(m, b, src, b.Min, draw.Src)
	return m
}

// ColorModel returns the Image's color model, which is always color.RGBAModel.
func (p *Image) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds returns the domain for which At can return a non-zero color.
func (p *Image) Bounds() image.Rectangle {
	return p.Rect
}

// At returns the color of the pixel at (x, y).
// It returns color.RGBA{} (transparent black) for points outside the image bounds.
func (p *Image) At(x, y int) color.Color {
	if !(image.Point{x, y}.In(p.Rect)) {
		return color.RGBA{}
	}
	i := p.PixOffset(x, y)
	// This type is non-premultiplied alpha, so we can return the raw values.
	return color.RGBA{R: p.Pix[i+0], G: p.Pix[i+1], B: p.Pix[i+2], A: p.Pix[i+3]}
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y).
func (p *Image) PixOffset(x, y int) int {
	return (y-p.Rect.Min.Y)*p.Stride + (x-p.Rect.Min.X)*4
}

// Set sets the color of the pixel at (x, y).
// It converts any given color.Color to the RGBA model.
func (p *Image) Set(x, y int, c color.Color) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	// If the color is already RGBA, we can avoid the conversion.
	// This is a common case and provides a small performance boost.
	if c, ok := c.(color.RGBA); ok {
		p.Pix[i+0] = c.R
		p.Pix[i+1] = c.G
		p.Pix[i+2] = c.B
		p.Pix[i+3] = c.A
		return
	}
	// Fallback to the standard conversion for other color types.
	c1 := color.RGBAModel.Convert(c).(color.RGBA)
	p.Pix[i+0] = c1.R
	p.Pix[i+1] = c1.G
	p.Pix[i+2] = c1.B
	p.Pix[i+3] = c1.A
}

// SetRGBA sets the color of the pixel at (x, y) to a specific RGBA value.
// This is a more direct and slightly more performant version of Set.
func (p *Image) SetRGBA(x, y int, c color.RGBA) {
	if !(image.Point{x, y}.In(p.Rect)) {
		return
	}
	i := p.PixOffset(x, y)
	p.Pix[i+0] = c.R
	p.Pix[i+1] = c.G
	p.Pix[i+2] = c.B
	p.Pix[i+3] = c.A
}

// SubImage returns an image representing the portion of the original image
// visible through r. The returned value shares pixels with the original image.
func (p *Image) SubImage(r image.Rectangle) image.Image {
	r = r.Intersect(p.Rect)
	// If r1 and r2 are Rectangles, r1.Intersect(r2) is not guaranteed to be
	// inside either r1 or r2 if the intersection is empty. Without explicitly
	// checking for this, the Pix[i:] expression below can panic.
	if r.Empty() {
		return &Image{}
	}
	i := p.PixOffset(r.Min.X, r.Min.Y)
	return &Image{
		Pix:    p.Pix[i:],
		Stride: p.Stride,
		Rect:   r,
	}
}

// Opaque scans the entire image and returns whether it is fully opaque.
// An image is opaque if all of its pixels have an alpha value of 255.
func (p *Image) Opaque() bool {
	if p.Rect.Empty() {
		return true
	}
	// Start at the alpha component of the first pixel.
	i0 := p.PixOffset(p.Rect.Min.X, p.Rect.Min.Y) + 3
	// Iterate through each row.
	for y := p.Rect.Min.Y; y < p.Rect.Max.Y; y++ {
		// Check the alpha value for each pixel in the row.
		for i := i0; i < i0+p.Rect.Dx()*4; i += 4 {
			if p.Pix[i] != 0xff {
				return false
			}
		}
		i0 += p.Stride
	}
	return true
}
