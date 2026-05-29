# Perceptual Hashing (dHash)

## What is it?

A perceptual hash is a fingerprint of an image, but unlike a cryptographic hash (SHA256, MD5),
it doesn't care about exact bytes. Two images that **look the same** to a human will produce
the **same or nearly the same** hash — even if they have different file sizes, resolutions,
compression levels, or formats.

## How dHash works

dHash stands for **difference hash**. It works in 3 simple steps:

### 1. Shrink to 9×8 pixels and convert to grayscale

No matter how big the original image is (4000×3000 or 400×300), it gets resized down to
just 9 pixels wide by 8 pixels tall. Color information is discarded — only brightness matters.

**Why 9×8?** The goal is a 64-bit hash (convenient: fits in a single CPU register, easy to
compare with XOR). With 8 rows, you need 8 comparisons per row to get 8 × 8 = 64 bits.
To make 8 comparisons across a row, the row must have **N+1 = 9** pixels (compare pixel 0→1,
1→2, … 7→8). That's the only reason: 9 pixels wide gives exactly 8 differences per row.

```
Original image (e.g. 4000×3000)    →    9×8 grayscale grid
┌──────────────────────┐                ┌─────────────────┐
│                      │                │ ▓ ░ ▓ ░ ▓ ░ ▓ ░ ▒ │
│   🌄 mountains       │                │ ░ ▒ ░ ▒ ▓ ▒ ░ ▓ ▒ │
│                      │                │ ▒ ░ ▓ ▒ ░ ▓ ▒ ░ ▓ │
│                      │                │ ... 5 more rows  ...│
└──────────────────────┘                └─────────────────┘
```

### 2. Compare each pixel to its right neighbor

For each of the 8 rows, we look at the first 8 pixels (index 0–7) and compare each one
to the pixel immediately to its right (index 1–8).

- If the left pixel is **darker** than the right pixel → write `1`
- If the left pixel is **lighter or equal** → write `0`

```
Row of 9 grayscale values:
  42   60   80   55   30   90   45   70   50
   ↓    ↓    ↓    ↓    ↓    ↓    ↓    ↓    ← 8 comparisons per row
  42<60 60<80 80>55 55>30 30<90 90>45 45<70 70>50
   1     1     0     0     1     0     1     0    → 8 bits per row

8 rows × 8 comparisons = 64-bit fingerprint
```

### 3. Build the 64-bit number

The 64 bits are packed into a single number like `0x9e3a7f1c5b2d8400`. This is the dHash.

## Comparing two hashes

Two images are similar if their hashes have a **small Hamming distance** — the number
of bit positions where they differ.

```
Hash A: 1100 1010 0011 1110 ...
Hash B: 1100 1000 0011 1110 ...
             ↑         (one bit differs)

Distance = 1 → these images are nearly identical
```

A distance of **0** means identical hashes. A distance ≤ **6** typically means the
same image at different quality levels or slightly cropped. Higher distances mean
increasingly different images.

## Why dHash?

| Hash type | Small edit to file | Resize image | Recompress JPEG |
|-----------|-------------------|--------------|-----------------|
| SHA256    | completely different | completely different | completely different |
| MD5       | completely different | completely different | completely different |
| **dHash** | **nearly identical** | **nearly identical** | **nearly identical** |

Cryptographic hashes are designed to change completely when even a single bit changes.
dHash is designed to stay stable across the kinds of transformations that happen to
images online (resizing, recompression, format conversion).

## Usage

```go
import "go.goblog.app/app/pkgs/phash"

// Hash an image
img, _ := png.Decode(file)
hash := phash.Hash(img) // returns uint64

// Compare two hashes
dist := phash.Distance(hash1, hash2) // returns int (0–64)

// Check if two images are likely the same
if dist <= 6 {
    // images are visually very similar
}
```

## Limitations

- **Not rotation-invariant**: a 90° rotated image will produce a completely different hash.

- **EXIF orientation is NOT handled by this package**: `phash.Hash` operates on raw pixel
  data from whatever `image.Image` you pass in. It doesn't know or care about EXIF.
  In GoBlog's migration, this is handled at a higher level: the migration code decodes
  each file **twice** — once with `imaging.AutoOrientation(true)` and once without —
  and stores both hashes. When comparing two files, it uses the minimum of the two
  distances.

- **Not crop-invariant**: a heavily cropped image will differ significantly.

- **Not a search index**: comparing every pair is O(n²). Pre-filter by aspect ratio
  (±15%) to make this practical for large collections.

- **Threshold tuning**: the right distance threshold (default 6) depends on how aggressive
  the compression is. Adjust if needed.
