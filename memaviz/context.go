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

func Compound(contexts ...Context) CompoundContextImpl {
	return CompoundContextImpl{contexts}
}

type CompoundContextImpl struct{ contexts []Context }

func (c CompoundContextImpl) Enter() {
	for i := range c.contexts {
		c.contexts[i].Enter()
	}
}
func (c CompoundContextImpl) Exit() {
	for i := range c.contexts {
		c.contexts[len(c.contexts)-i-1].Exit()
	}
}

type Bindable interface {
	Bind()
	Unbind()
}

type Framebuffer struct{ Bindable }

func (b Framebuffer) Enter() { b.Bind() }
func (b Framebuffer) Exit()  { b.Unbind() }

type BindableOneArg interface {
	Bind(gl.GLenum)
	Unbind(gl.GLenum)
}

type Texture struct {
	BindableOneArg
	Value gl.GLenum
}

func (b Texture) Enter() { gl.Enable(gl.TEXTURE_2D); b.Bind(b.Value) }
func (b Texture) Exit()  { b.Unbind(b.Value); gl.Disable(gl.TEXTURE_2D) }

// A context which preserves the matrix mode, drawing, etc.
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

type WindowCoords struct{}

func (wc WindowCoords) Enter() {
	w, h := GetViewportWH()
	Matrix{gl.PROJECTION}.Enter()
	gl.LoadIdentity()
	gl.Ortho(0, float64(w), float64(h), 0, -1, 1)
	Matrix{gl.MODELVIEW}.Enter()
	gl.LoadIdentity()
}

func (wc WindowCoords) Exit() {
	Matrix{gl.MODELVIEW}.Exit()
	Matrix{gl.PROJECTION}.Exit()
}
