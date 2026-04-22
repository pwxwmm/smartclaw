package native

import (
	"math"
	"testing"
)

func TestNewColor(t *testing.T) {
	c := NewColor(255, 128, 0)
	if math.Abs(c.R-1.0) > 1e-9 {
		t.Errorf("R = %v, want 1.0", c.R)
	}
	if math.Abs(c.G-128.0/255.0) > 1e-9 {
		t.Errorf("G = %v, want %v", c.G, 128.0/255.0)
	}
	if math.Abs(c.B-0.0) > 1e-9 {
		t.Errorf("B = %v, want 0.0", c.B)
	}
}

func TestParseHex(t *testing.T) {
	t.Run("6-char hex", func(t *testing.T) {
		c := ParseHex("#ff0000")
		if math.Abs(c.R-1.0) > 1e-9 || math.Abs(c.G) > 1e-9 || math.Abs(c.B) > 1e-9 {
			t.Errorf("ParseHex(#ff0000) = %v, want red", c)
		}
	})

	t.Run("without hash", func(t *testing.T) {
		c := ParseHex("00ff00")
		if math.Abs(c.R) > 1e-9 || math.Abs(c.G-1.0) > 1e-9 || math.Abs(c.B) > 1e-9 {
			t.Errorf("ParseHex(00ff00) = %v, want green", c)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		c := ParseHex("")
		if c.R != 0 || c.G != 0 || c.B != 0 {
			t.Errorf("ParseHex('') = %v, want zero Color", c)
		}
	})

	t.Run("3-char hex returns zero (only 6-char supported)", func(t *testing.T) {
		c := ParseHex("#f00")
		if c.R != 0 || c.G != 0 || c.B != 0 {
			t.Errorf("ParseHex(#f00) = %v, want zero Color (3-char not supported)", c)
		}
	})
}

func TestColorDiff(t *testing.T) {
	t.Run("same color", func(t *testing.T) {
		c := NewColor(128, 128, 128)
		if d := ColorDiff(c, c); d != 0 {
			t.Errorf("ColorDiff(same, same) = %v, want 0", d)
		}
	})

	t.Run("max difference", func(t *testing.T) {
		black := NewColor(0, 0, 0)
		white := NewColor(255, 255, 255)
		d := ColorDiff(black, white)
		if math.Abs(d-1.0) > 1e-9 {
			t.Errorf("ColorDiff(black, white) = %v, want ~1.0", d)
		}
	})
}

func TestDistance(t *testing.T) {
	c1 := NewColor(100, 100, 100)
	c2 := NewColor(200, 100, 100)
	d := c1.Distance(c2)
	if d <= 0 {
		t.Errorf("Distance() = %v, want > 0", d)
	}
}

func TestToRGB_RoundTrip(t *testing.T) {
	r, g, b := uint8(200), uint8(100), uint8(50)
	c := NewColor(r, g, b)
	rr, gg, bb := c.ToRGB()
	if rr != r || gg != g || bb != b {
		t.Errorf("ToRGB round-trip: got (%d,%d,%d), want (%d,%d,%d)", rr, gg, bb, r, g, b)
	}
}

func TestToHex_RoundTrip(t *testing.T) {
	hex := "#ff8844"
	c := ParseHex(hex)
	got := c.ToHex()
	if got != hex {
		t.Errorf("ToHex round-trip: got %q, want %q", got, hex)
	}
}

func TestClosest(t *testing.T) {
	red := NewColor(255, 0, 0)
	green := NewColor(0, 255, 0)
	blue := NewColor(0, 0, 255)
	palette := NewPalette(red, green, blue)

	t.Run("finds nearest", func(t *testing.T) {
		target := NewColor(200, 10, 10)
		closest := palette.Closest(target)
		if closest != red {
			t.Errorf("Closest() = %v, want red", closest)
		}
	})

	t.Run("empty palette returns zero Color", func(t *testing.T) {
		empty := NewPalette()
		c := empty.Closest(NewColor(128, 128, 128))
		if c != (Color{}) {
			t.Errorf("Closest() on empty = %v, want zero Color", c)
		}
	})
}

func TestSortByDistance(t *testing.T) {
	red := NewColor(255, 0, 0)
	green := NewColor(0, 255, 0)
	blue := NewColor(0, 0, 255)
	palette := NewPalette(green, blue, red)

	target := NewColor(250, 5, 5)
	sorted := palette.SortByDistance(target)

	if sorted[0] != red {
		t.Errorf("SortByDistance()[0] = %v, want red (closest to target)", sorted[0])
	}
	if len(sorted) != 3 {
		t.Errorf("SortByDistance() len = %d, want 3", len(sorted))
	}
}
