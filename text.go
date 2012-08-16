package main

import (
	"image"
	"image/draw"
	"io/ioutil"
	"log"
	
	"github.com/banthar/gl"
	"code.google.com/p/freetype-go/freetype"
)

type Text struct {
	str string
	w, h int
	id gl.Texture
}

func MakeText(str string, size float64) Text {

	var text Text
	text.str = str

	fontBytes, err := ioutil.ReadFile("env/src/code.google.com/p/freetype-go/luxi-fonts/luximr.ttf")
	if err != nil {
		log.Panic(err)
	}
	font, err := freetype.ParseFont(fontBytes)
	if err != nil {
		log.Panic(err)
	}

	// TODO: Two passes with DrawString, so that the 
	fg, bg := image.White, image.Black
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(size)
	
	pt := freetype.Pt(10, 10+int(c.PointToFix32(size)>>8))
	s, err := c.DrawString(text.str, pt)
    if err != nil { log.Panic("Error: ", err) }
	
	text.w, text.h = int(s.X/256), int(s.Y/256)+10
	
	rgba := image.NewRGBA(image.Rect(0, 0, text.w, text.h))
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(fg)
	
    _, err = c.DrawString(text.str, pt)
    if err != nil { log.Panic("Error: ", err) }

	text.id = gl.GenTexture()
	text.id.Bind(gl.TEXTURE_2D)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, text.w, text.h, 0, gl.RGBA, 
				  gl.UNSIGNED_BYTE, rgba.Pix)

	if gl.GetError() != gl.NO_ERROR {
		text.id.Delete()
		log.Panic("Failed to load a texture, err = ", gl.GetError())
	}
	
	return text
}

func (text *Text) Draw(x, y int) {
	var w, h int = text.w, text.h
	var u, v, u2, v2 float32 = 0, 1, 1, 0
	text.id.Bind(gl.TEXTURE_2D)
	gl.Begin(gl.QUADS)

	gl.TexCoord2f(float32(u), float32(v))
	gl.Vertex2i(int(x), int(y))

	gl.TexCoord2f(float32(u2), float32(v))
	gl.Vertex2i(int(x+w), int(y))

	gl.TexCoord2f(float32(u2), float32(v2))
	gl.Vertex2i(int(x+w), int(y+h))

	gl.TexCoord2f(float32(u), float32(v2))
	gl.Vertex2i(int(x), int(y+h))

	gl.End()
}
