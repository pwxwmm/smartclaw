package native

import (
	"math"
)

type Color struct {
	R, G, B float64
}

func NewColor(r, g, b uint8) Color {
	return Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
	}
}

func ParseHex(hex string) Color {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}

	var r, g, b uint8
	if len(hex) == 6 {
		r = hexToByte(hex[0:2])
		g = hexToByte(hex[2:4])
		b = hexToByte(hex[4:6])
	}

	return NewColor(r, g, b)
}

func hexToByte(hex string) uint8 {
	var val uint8
	for _, c := range hex {
		val <<= 4
		switch {
		case c >= '0' && c <= '9':
			val |= uint8(c - '0')
		case c >= 'a' && c <= 'f':
			val |= uint8(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			val |= uint8(c - 'A' + 10)
		}
	}
	return val
}

func ColorDiff(c1, c2 Color) float64 {
	dr := c1.R - c2.R
	dg := c1.G - c2.G
	db := c1.B - c2.B

	return math.Sqrt(dr*dr+dg*dg+db*db) / math.Sqrt(3.0)
}

func (c Color) Distance(other Color) float64 {
	return ColorDiff(c, other)
}

func (c Color) ToRGB() (uint8, uint8, uint8) {
	return uint8(c.R * 255), uint8(c.G * 255), uint8(c.B * 255)
}

func (c Color) ToHex() string {
	r, g, b := c.ToRGB()
	return formatHex(r, g, b)
}

func formatHex(r, g, b uint8) string {
	const hexChars = "0123456789abcdef"
	return "#" + string(hexChars[r>>4]) + string(hexChars[r&0x0f]) +
		string(hexChars[g>>4]) + string(hexChars[g&0x0f]) +
		string(hexChars[b>>4]) + string(hexChars[b&0x0f])
}

type ColorPalette []Color

func NewPalette(colors ...Color) ColorPalette {
	return ColorPalette(colors)
}

func (p ColorPalette) Closest(target Color) Color {
	if len(p) == 0 {
		return Color{}
	}

	minDist := math.MaxFloat64
	var closest Color

	for _, c := range p {
		dist := target.Distance(c)
		if dist < minDist {
			minDist = dist
			closest = c
		}
	}

	return closest
}

func (p ColorPalette) SortByDistance(target Color) ColorPalette {
	sorted := make(ColorPalette, len(p))
	copy(sorted, p)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Distance(target) > sorted[j].Distance(target) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
