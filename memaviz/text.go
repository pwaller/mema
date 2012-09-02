package main

import (
	"image"
	"image/draw"
	"io/ioutil"
	"log"
	
	"github.com/banthar/gl"
	"code.google.com/p/freetype-go/freetype"
)

var FontFile = "env/src/code.google.com/p/freetype-go/luxi-fonts/luximr.ttf"

type Text struct {
	str string
	w, h int
	id gl.Texture
}

func MakeText(str string, size float64) *Text {
	defer OpenGLSentinel()()

	text := &Text{}
	text.str = str

	// TODO: Something if font doesn't exist
	fontBytes, err := ioutil.ReadFile(FontFile)
	if err != nil {
		log.Panic(err)
	}
	font, err := freetype.ParseFont(fontBytes)
	if err != nil {
		log.Panic(err)
	}
	
	fg, bg := image.White, image.Black
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(font)
	c.SetFontSize(size)
	
	pt := freetype.Pt(10, 10+int(c.PointToFix32(size)>>8))
	s, err := c.DrawString(text.str, pt)
	if err != nil { log.Panic("Error: ", err) }
	
	text.w, text.h = int(s.X/256), int(s.Y/256)+10
	
	if text.w > 4096 {
		text.w = 4096
	}
	
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
		log.Panic("Failed to load a texture, err = ", gl.GetError(),
				  " str = ", str, " w = ", text.w, " h = ", text.h)
	}
		
	return text
}

func (text *Text) destroy() {
	text.id.Delete()
}

func (text *Text) Draw(x, y int) {
	defer OpenGLSentinel()()
	
	var w, h int = text.w / 2, text.h / 2
	var u, v, u2, v2 float32 = 0, 1, 1, 0
	text.id.Bind(gl.TEXTURE_2D)
	
	gl.Begin(gl.QUADS)

	gl.TexCoord2f(u, v)
	gl.Vertex2i(x, y)

	gl.TexCoord2f(u2, v)
	gl.Vertex2i(x+w, y)

	gl.TexCoord2f(u2, v2)
	gl.Vertex2i(x+w, y+h)
	
	gl.TexCoord2f(u, v2)
	gl.Vertex2i(x, y+h)

	gl.End()
}