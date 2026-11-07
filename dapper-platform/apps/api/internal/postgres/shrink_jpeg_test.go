package postgres

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestShrinkJPEG(t *testing.T) {
	var big bytes.Buffer
	if err := jpeg.Encode(&big, image.NewRGBA(image.Rect(0, 0, 1600, 1200)), nil); err != nil {
		t.Fatal(err)
	}
	out, err := shrinkJPEG(big.Bytes(), 360)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() > 360 || img.Bounds().Dy() > 360 || img.Bounds().Dx() < 200 || len(out) >= big.Len() {
		t.Fatal("thumbnail wrong", err, img.Bounds(), len(out), big.Len())
	}
	if _, err = shrinkJPEG([]byte("not an image"), 360); err == nil {
		t.Fatal("a non-image produced a thumbnail")
	}
}
