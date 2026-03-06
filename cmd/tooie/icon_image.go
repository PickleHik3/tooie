package main

import (
	"image"
	stdraw "image/draw"
)

func trimTransparentImage(src image.Image) image.Image {
	if src == nil {
		return nil
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return src
	}

	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X-1, b.Min.Y-1
	found := false

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if !found {
				minX, maxX = x, x
				minY, maxY = y, y
				found = true
				continue
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	if !found {
		return src
	}

	trimmedBounds := image.Rect(0, 0, maxX-minX+1, maxY-minY+1)
	dst := image.NewRGBA(trimmedBounds)
	stdraw.Draw(dst, trimmedBounds, src, image.Point{X: minX, Y: minY}, stdraw.Src)
	return dst
}
