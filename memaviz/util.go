package main

import (
	"image"
	"image/png"
	"log"
	"os"
	"sort"

	"github.com/banthar/gl"
)

type UInt64Slice []uint64

func (p UInt64Slice) Len() int           { return len(p) }
func (p UInt64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p UInt64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p UInt64Slice) Sort() { sort.Sort(p) }

func min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}

func Capture() {
	// TODO: co-ordinates, filename, cleverness to stitch many together
	im := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	gl.ReadBuffer(gl.BACK_LEFT)
	gl.ReadPixels(0, 0, 400, 400, gl.RGBA, im.Pix)

	fd, err := os.Create("test.png")
	if err != nil {
		log.Panic("Err: ", err)
	}
	defer fd.Close()

	png.Encode(fd, im)
}
