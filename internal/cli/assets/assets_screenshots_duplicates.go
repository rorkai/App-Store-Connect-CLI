package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

type screenshotPixelDigest [sha256.Size]byte

func decodedScreenshotPixelDigest(path string) (screenshotPixelDigest, error) {
	img, err := decodeScreenshotPixels(path)
	if err != nil {
		return screenshotPixelDigest{}, err
	}

	return screenshotPixelDigestFromImage(img), nil
}

func screenshotPixelDigestFromImage(img image.Image) screenshotPixelDigest {
	bounds := img.Bounds()
	hasher := sha256.New()
	var dimensions [16]byte
	binary.BigEndian.PutUint64(dimensions[0:8], uint64(bounds.Dx()))
	binary.BigEndian.PutUint64(dimensions[8:16], uint64(bounds.Dy()))
	_, _ = hasher.Write(dimensions[:])

	row := make([]byte, bounds.Dx()*8)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		normalizeScreenshotPixelRow(row, img, y)
		_, _ = hasher.Write(row)
	}

	var digest screenshotPixelDigest
	copy(digest[:], hasher.Sum(nil))
	return digest
}

func decodedScreenshotPixelsEqual(leftPath, rightPath string) (bool, error) {
	left, err := decodeScreenshotPixels(leftPath)
	if err != nil {
		return false, err
	}
	right, err := decodeScreenshotPixels(rightPath)
	if err != nil {
		return false, err
	}

	return screenshotPixelsEqual(left, right), nil
}

func screenshotPixelsEqual(left, right image.Image) bool {
	leftBounds := left.Bounds()
	rightBounds := right.Bounds()
	if leftBounds.Dx() != rightBounds.Dx() || leftBounds.Dy() != rightBounds.Dy() {
		return false
	}

	leftRow := make([]byte, leftBounds.Dx()*8)
	rightRow := make([]byte, rightBounds.Dx()*8)
	for offsetY := 0; offsetY < leftBounds.Dy(); offsetY++ {
		normalizeScreenshotPixelRow(leftRow, left, leftBounds.Min.Y+offsetY)
		normalizeScreenshotPixelRow(rightRow, right, rightBounds.Min.Y+offsetY)
		if !bytes.Equal(leftRow, rightRow) {
			return false
		}
	}
	return true
}

func decodeScreenshotPixels(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image pixels for %q: %w", path, err)
	}
	return img, nil
}

func normalizeScreenshotPixelRow(dst []byte, img image.Image, y int) {
	bounds := img.Bounds()
	for offsetX := 0; offsetX < bounds.Dx(); offsetX++ {
		pixel := color.NRGBA64Model.Convert(img.At(bounds.Min.X+offsetX, y)).(color.NRGBA64)
		index := offsetX * 8
		binary.BigEndian.PutUint16(dst[index:index+2], pixel.R)
		binary.BigEndian.PutUint16(dst[index+2:index+4], pixel.G)
		binary.BigEndian.PutUint16(dst[index+4:index+6], pixel.B)
		binary.BigEndian.PutUint16(dst[index+6:index+8], pixel.A)
	}
}
