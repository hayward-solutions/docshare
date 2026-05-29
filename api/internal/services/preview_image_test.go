package services

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestIsThumbnailableImage(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/gif", true},
		{"image/webp", true},
		{"image/bmp", true},
		{"image/tiff", true},
		{"image/svg+xml", false},
		{"image/heic", false},
		{"image/heif", false},
		{"image/avif", false},
		{"application/pdf", false},
		{"text/plain", false},
		{"", false},
		{"IMAGE/PNG", false}, // case-sensitive; mimes are normalized lowercase
	}
	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			if got := IsThumbnailableImage(tt.mime); got != tt.want {
				t.Errorf("IsThumbnailableImage(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func makeTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

func TestResizeImageToJPEG_BoundsLargerSide(t *testing.T) {
	// Wide image: 1200x300 → fit into 400×400 means width=400, height=100.
	src := makeTestPNG(t, 1200, 300)

	out, err := resizeImageToJPEG(bytes.NewReader(src), 400, 80)
	if err != nil {
		t.Fatalf("resizeImageToJPEG: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not a valid JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 400 {
		t.Errorf("expected width=400, got %d", b.Dx())
	}
	if b.Dy() != 100 {
		t.Errorf("expected height=100 (aspect-preserved), got %d", b.Dy())
	}
}

func TestResizeImageToJPEG_DoesNotUpscale(t *testing.T) {
	// 50x50 with maxDim=400 should stay at 50x50 (imaging.Fit doesn't
	// upscale; preserves the original when it already fits).
	src := makeTestPNG(t, 50, 50)

	out, err := resizeImageToJPEG(bytes.NewReader(src), 400, 80)
	if err != nil {
		t.Fatalf("resizeImageToJPEG: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not a valid JPEG: %v", err)
	}
	if w, h := img.Bounds().Dx(), img.Bounds().Dy(); w != 50 || h != 50 {
		t.Errorf("expected 50x50 (no upscale), got %dx%d", w, h)
	}
}

func TestResizeImageToJPEG_RejectsGarbage(t *testing.T) {
	_, err := resizeImageToJPEG(bytes.NewReader([]byte("not an image, just text bytes")), 400, 80)
	if err == nil {
		t.Fatal("expected decode error on garbage input")
	}
}

// TestResizeImageToJPEG_RejectsPixelBomb feeds in a PNG whose IHDR claims a
// 20000×20000 canvas. DecodeConfig succeeds (it reads the header), the size
// check fires, and we never reach the full decode that would allocate
// ~1.6 GB. Verifies the OOM guard.
func TestResizeImageToJPEG_RejectsPixelBomb(t *testing.T) {
	src := makeTestPNG(t, 32, 32) // valid small PNG we'll rewrite the dims of

	// PNG layout: 8-byte signature, then chunks. IHDR is the first chunk
	// and stores width (4B big-endian) at byte offset 16 and height at 20.
	// The IHDR length and CRC don't change so we can just splice the dims.
	bigW, bigH := uint32(20000), uint32(20000)
	binary.BigEndian.PutUint32(src[16:20], bigW)
	binary.BigEndian.PutUint32(src[20:24], bigH)
	// Recompute the IHDR CRC (covers the type tag + 13-byte body, offsets
	// 12..29) so DecodeConfig accepts the chunk.
	src[29-4], src[29-3], src[29-2], src[29-1] = 0, 0, 0, 0 // placeholder
	crc := crc32.ChecksumIEEE(src[12:29])
	binary.BigEndian.PutUint32(src[29:33], crc)

	_, err := resizeImageToJPEG(bytes.NewReader(src), 400, 80)
	if err == nil {
		t.Fatal("expected pixel-bomb rejection")
	}
	if !strings.Contains(err.Error(), "exceed max") {
		t.Errorf("expected size-cap error, got %v", err)
	}
}
