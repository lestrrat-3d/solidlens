package solidlens_test

import (
	"math"
	"testing"

	"github.com/lestrrat-3d/solidlens"
	"github.com/stretchr/testify/require"
)

func TestColorNRGBAMatchesTransferFunction(t *testing.T) {
	check := func(value float64) {
		t.Helper()
		clamped := math.Min(math.Max(value, 0), 1)
		var want uint8
		if clamped <= 0.0031308 {
			want = uint8(clamped*12.92*255 + 0.5)
		} else {
			want = uint8((1.055*math.Pow(clamped, 1/2.4)-0.055)*255 + 0.5)
		}
		got := solidlens.RGB(value, value, value).NRGBA()
		require.Equal(t, want, got.R, "red channel at %g", value)
		require.Equal(t, want, got.G, "green channel at %g", value)
		require.Equal(t, want, got.B, "blue channel at %g", value)
	}

	for index := range 65537 {
		check(float64(index) / 65536)
	}
	for channel := 1; channel < 256; channel++ {
		encoded := (float64(channel) - 0.5) / 255
		linear := encoded / 12.92
		if encoded > 0.04045 {
			linear = math.Pow((encoded+0.055)/1.055, 2.4)
		}
		check(math.Nextafter(linear, 0))
		check(linear)
		check(math.Nextafter(linear, 1))
	}
	check(-1)
	check(2)
	check(math.NaN())
	check(math.Inf(-1))
	check(math.Inf(1))
}
