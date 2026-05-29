package phash

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func createSolidImage(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func createGradientImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(x * 255 / w)
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

func createCheckerboard(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	size := w / 8
	for y := range h {
		for x := range w {
			if ((x/size)+(y/size))%2 == 0 {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
		}
	}
	return img
}

func savePNG(t *testing.T, img image.Image, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestHashSolidIdentical(t *testing.T) {
	a := createSolidImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	b := createSolidImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	ha := Hash(a)
	hb := Hash(b)
	if ha != hb {
		t.Errorf("identical solids should hash to same value: %064b != %064b", ha, hb)
	}
	if d := Distance(ha, hb); d != 0 {
		t.Errorf("distance between identical hashes should be 0, got %d", d)
	}
}

func TestHashDifferentSolids(t *testing.T) {
	a := createSolidImage(800, 600, color.Black)
	b := createSolidImage(800, 600, color.White)
	ha := Hash(a)
	hb := Hash(b)
	if ha != hb {
		t.Errorf("solid black and solid white should hash the same (both flat): %064b != %064b", ha, hb)
	}
}

func TestHashSameImageDifferentSizes(t *testing.T) {
	a := createGradientImage(800, 600)
	b := createGradientImage(400, 300)
	ha := Hash(a)
	hb := Hash(b)
	d := Distance(ha, hb)
	if d > 10 {
		t.Errorf("same gradient at different sizes should have low distance, got %d", d)
	}
}

func TestHashDifferentPatterns(t *testing.T) {
	a := createGradientImage(800, 600)
	b := createCheckerboard(800, 600)
	ha := Hash(a)
	hb := Hash(b)
	d := Distance(ha, hb)
	if d < 20 {
		t.Errorf("different patterns should have high distance, got %d", d)
	}
}

func TestHashDeterministic(t *testing.T) {
	img := createGradientImage(800, 600)
	h1 := Hash(img)
	h2 := Hash(img)
	if h1 != h2 {
		t.Errorf("hash should be deterministic: %064b != %064b", h1, h2)
	}
}

func TestDistanceSelf(t *testing.T) {
	h := uint64(0xDEADBEEFCAFEBABE)
	if d := Distance(h, h); d != 0 {
		t.Errorf("distance of hash with itself should be 0, got %d", d)
	}
}

func TestDistanceKnown(t *testing.T) {
	a := uint64(0b0000_0000_0000_0000_0000)
	b := uint64(0b0000_0000_0000_0000_0001)
	if d := Distance(a, b); d != 1 {
		t.Errorf("distance between 0 and 1 should be 1, got %d", d)
	}
	if d := Distance(a, a); d != 0 {
		t.Errorf("distance between same values should be 0, got %d", d)
	}
}

func TestHashEmptyImage(t *testing.T) {
	img := createSolidImage(1, 1, color.Black)
	h := Hash(img)
	_ = h // should not panic
}

func TestHashPNGDecoded(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/test.png"
	img1 := createGradientImage(800, 600)
	savePNG(t, img1, path)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	h1 := Hash(img1)
	h2 := Hash(decoded)
	if h1 != h2 {
		t.Errorf("hash should be same after PNG encode/decode: %064b != %064b", h1, h2)
	}
}

func TestHashNoPanicOnSubImage(t *testing.T) {
	base := createGradientImage(800, 600)
	sub := base.(*image.RGBA).SubImage(image.Rect(100, 100, 300, 300))
	h := Hash(sub)
	_ = h // should not panic
}

func BenchmarkHash(b *testing.B) {
	img := createGradientImage(4000, 3000)
	b.ResetTimer()
	for range b.N {
		Hash(img)
	}
}
