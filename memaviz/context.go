package main

import (
	"log"

	"github.com/banthar/gl"
)

type Context interface {
	Enter()
	Exit()
}

func With(c Context, action func()) {
	c.Enter()
	defer c.Exit()
	action()
}

type Texture struct {
	Object interface {
		Bind(gl.GLenum)
		Unbind(gl.GLenum)
	}
	Value gl.GLenum
}

func (b Texture) Enter() { b.Object.Bind(b.Value) }
func (b Texture) Exit()  { b.Object.Unbind(b.Value) }

type Matrix struct{ Type gl.GLenum }

func (m Matrix) Enter() {
	gl.PushAttrib(gl.TRANSFORM_BIT)
	gl.MatrixMode(m.Type)
	gl.PushMatrix()
}

func (m Matrix) Exit() {
	gl.PopMatrix()
	gl.PopAttrib()
}

type Attrib struct{ Bits gl.GLbitfield }

func (a Attrib) Enter() {
	gl.PushAttrib(a.Bits)
	e := gl.GetError()
	if e != gl.NO_ERROR {
		log.Panic("Bad gl.PushAttrib(), reason: ", e)
	}
}

func (a Attrib) Exit() {
	gl.PopAttrib()
	e := gl.GetError()
	if e != gl.NO_ERROR {
		log.Panic("Bad gl.PopAttrib(), reason: ", e)
	}
}

type Primitive struct{ Type gl.GLenum }

func (p Primitive) Enter() { gl.Begin(p.Type) }
func (p Primitive) Exit()  { gl.End() }
