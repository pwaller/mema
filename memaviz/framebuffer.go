package main

import (
	"image"
	"log"

	"github.com/banthar/gl"
)

// Mapping from texture dimensions onto ready made framebuffer/renderbuffer
// therefore we only construct one per image dimensions
// This number should be less than O(1000) otherwise opengl throws OUT_OF_MEMORY
// on some cards
var framebuffers map[image.Point]*fborbo = make(map[image.Point]*fborbo)

type fborbo struct {
	fbo gl.Framebuffer
	rbo gl.Renderbuffer
}

func GetFBORBO(t *Texture) *fborbo {
	p := image.Point{t.w, t.h}
	result, ok := framebuffers[p]
	if ok {
		return result
	}

	result = &fborbo{}

	result.rbo = gl.GenRenderbuffer()
	OpenGLSentinel()
	result.fbo = gl.GenFramebuffer()
	OpenGLSentinel()

	result.fbo.Bind()

	result.rbo.Bind()
	gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT, t.w, t.h)
	result.rbo.Unbind()

	result.rbo.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER)

	result.fbo.Unbind()

	framebuffers[image.Point{t.w, t.h}] = result
	return result
}

type Framebuffer struct {
	tex *Texture
	*fborbo
}

func (b *Framebuffer) Enter() {
	if b.fborbo == nil {
		b.fborbo = GetFBORBO(b.tex)
	}

	b.fbo.Bind()

	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, b.tex.Texture, 0)

	s := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
	if s != gl.FRAMEBUFFER_COMPLETE {
		log.Panicf("Incomplete framebuffer, reason: %x", s)
	}
}

func (b *Framebuffer) Exit() {
	b.fbo.Unbind()
}
