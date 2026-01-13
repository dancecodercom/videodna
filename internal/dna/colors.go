package dna

import "image/color"

// AverageColor returns the average RGB color of a row.
func AverageColor(row []byte, width int) color.Color {
	var rSum, gSum, bSum uint64
	for x := 0; x < width; x++ {
		i := x * 3
		rSum += uint64(row[i])
		gSum += uint64(row[i+1])
		bSum += uint64(row[i+2])
	}
	n := uint64(width)
	return color.RGBA{R: uint8(rSum / n), G: uint8(gSum / n), B: uint8(bSum / n), A: 255}
}

// MinColor returns the minimum RGB values in a row.
func MinColor(row []byte, width int) color.Color {
	var rMin, gMin, bMin uint8 = 255, 255, 255
	for x := 0; x < width; x++ {
		i := x * 3
		if row[i] < rMin {
			rMin = row[i]
		}
		if row[i+1] < gMin {
			gMin = row[i+1]
		}
		if row[i+2] < bMin {
			bMin = row[i+2]
		}
	}
	return color.RGBA{R: rMin, G: gMin, B: bMin, A: 255}
}

// MaxColor returns the maximum RGB values in a row.
func MaxColor(row []byte, width int) color.Color {
	var rMax, gMax, bMax uint8
	for x := 0; x < width; x++ {
		i := x * 3
		if row[i] > rMax {
			rMax = row[i]
		}
		if row[i+1] > gMax {
			gMax = row[i+1]
		}
		if row[i+2] > bMax {
			bMax = row[i+2]
		}
	}
	return color.RGBA{R: rMax, G: gMax, B: bMax, A: 255}
}

// MostCommonColor returns the most frequent color in a row.
func MostCommonColor(row []byte, width int) color.Color {
	colorCount := make(map[uint32]int)
	for x := 0; x < width; x++ {
		i := x * 3
		packed := uint32(row[i])<<16 | uint32(row[i+1])<<8 | uint32(row[i+2])
		colorCount[packed]++
	}

	var maxCount int
	var mostCommon uint32
	for col, count := range colorCount {
		if count > maxCount {
			maxCount = count
			mostCommon = col
		}
	}

	return color.RGBA{
		R: uint8((mostCommon >> 16) & 0xFF),
		G: uint8((mostCommon >> 8) & 0xFF),
		B: uint8(mostCommon & 0xFF),
		A: 255,
	}
}

// AverageColorCol returns the average RGB color of a column.
func AverageColorCol(buf []byte, col, width, height int) color.Color {
	var rSum, gSum, bSum uint64
	for y := 0; y < height; y++ {
		i := (y*width + col) * 3
		rSum += uint64(buf[i])
		gSum += uint64(buf[i+1])
		bSum += uint64(buf[i+2])
	}
	n := uint64(height)
	return color.RGBA{R: uint8(rSum / n), G: uint8(gSum / n), B: uint8(bSum / n), A: 255}
}

// MinColorCol returns the minimum RGB values in a column.
func MinColorCol(buf []byte, col, width, height int) color.Color {
	var rMin, gMin, bMin uint8 = 255, 255, 255
	for y := 0; y < height; y++ {
		i := (y*width + col) * 3
		if buf[i] < rMin {
			rMin = buf[i]
		}
		if buf[i+1] < gMin {
			gMin = buf[i+1]
		}
		if buf[i+2] < bMin {
			bMin = buf[i+2]
		}
	}
	return color.RGBA{R: rMin, G: gMin, B: bMin, A: 255}
}

// MaxColorCol returns the maximum RGB values in a column.
func MaxColorCol(buf []byte, col, width, height int) color.Color {
	var rMax, gMax, bMax uint8
	for y := 0; y < height; y++ {
		i := (y*width + col) * 3
		if buf[i] > rMax {
			rMax = buf[i]
		}
		if buf[i+1] > gMax {
			gMax = buf[i+1]
		}
		if buf[i+2] > bMax {
			bMax = buf[i+2]
		}
	}
	return color.RGBA{R: rMax, G: gMax, B: bMax, A: 255}
}

// MostCommonColorCol returns the most frequent color in a column.
func MostCommonColorCol(buf []byte, col, width, height int) color.Color {
	colorCount := make(map[uint32]int)
	for y := 0; y < height; y++ {
		i := (y*width + col) * 3
		packed := uint32(buf[i])<<16 | uint32(buf[i+1])<<8 | uint32(buf[i+2])
		colorCount[packed]++
	}

	var maxCount int
	var mostCommon uint32
	for c, count := range colorCount {
		if count > maxCount {
			maxCount = count
			mostCommon = c
		}
	}

	return color.RGBA{
		R: uint8((mostCommon >> 16) & 0xFF),
		G: uint8((mostCommon >> 8) & 0xFF),
		B: uint8(mostCommon & 0xFF),
		A: 255,
	}
}
