package main

import (
	"image"

	"github.com/banthar/gl"
)

type Texture struct {
	gl.Texture
	w, h int
}

func NewTexture(w, h int) *Texture {
	texture := &Texture{gl.GenTexture(), w, h}
	With(texture, func() {
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
	})
	return texture
}

func (t *Texture) Init() {
	With(t, func() {
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, t.w, t.h, 0, gl.RGBA,
			gl.UNSIGNED_BYTE, nil)
	})
}

func (b Texture) Enter() {
	gl.PushAttrib(gl.ENABLE_BIT)
	gl.Enable(gl.TEXTURE_2D)
	b.Bind(gl.TEXTURE_2D)
}
func (b Texture) Exit() {
	b.Unbind(gl.TEXTURE_2D)
	gl.PopAttrib()
}

func (t *Texture) AsImage() *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, t.w, t.h))
	With(t, func() {
		gl.GetTexImage(gl.TEXTURE_2D, 0, gl.RGBA, rgba.Pix)
	})
	return rgba
}
