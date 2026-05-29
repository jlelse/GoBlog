// Package phash provides perceptual image hashing (dHash).
package phash

import (
	"image"
	"image/color"
	"math/bits"
)

// Hash computes a 64-bit dHash (difference hash) from the given image.
//
// dHash works by resizing the image to 9×8 grayscale, then comparing
// each pixel to its right neighbor. If the left pixel is darker than
// the right, bit 1 is set; otherwise bit 0. This produces a 64-bit
// fingerprint (8 comparisons per row × 8 rows).
func Hash(img image.Image) uint64 {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	gray := image.NewGray(image.Rect(0, 0, 9, 8))

	for y := range 8 {
		for x := range 9 {
			srcX := bounds.Min.X + x*srcW/9
			srcY := bounds.Min.Y + y*srcH/8
			gray.Set(x, y, color.GrayModel.Convert(img.At(srcX, srcY)))
		}
	}

	var h uint64
	for y := range 8 {
		for x := range 8 {
			if gray.GrayAt(x, y).Y < gray.GrayAt(x+1, y).Y {
				h |= 1 << uint(y*8+x)
			}
		}
	}
	return h
}

// Distance returns the Hamming distance (number of differing bits)
// between two dHash values. A distance of 0 means identical hashes.
func Distance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}
