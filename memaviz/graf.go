package main

import (
	"log"
	"runtime"
	"strings"

	"github.com/banthar/gl"
	"github.com/jteeuwen/glfw"
)

func Init() {
	//gl.Enable(gl.DEPTH_TEST)

	// Anti-aliasing
	gl.Enable(gl.LINE_SMOOTH)
	gl.Enable(gl.BLEND)
	//gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.BlendFunc(gl.SRC_ALPHA, gl.DST_ALPHA)
	//gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.ONE)
	gl.Hint(gl.LINE_SMOOTH_HINT, gl.NICEST)

	extensions := gl.GetString(gl.EXTENSIONS)
	if !strings.Contains(extensions, "GL_ARB_framebuffer_object") {
		log.Panic("No FBO support :-(")
	}
}

func make_window(w, h int, title string) func() {
	// Required to make sure that the OpenGL go-routine doesn't get switched
	// to another thread (=> kerblammo)
	runtime.LockOSThread()

	if err := glfw.Init(); err != nil {
		log.Panic("glfw Error:", err)
	}

	err := glfw.OpenWindow(w, h, 0, 0, 0, 0, 0, 0, glfw.Windowed)
	if err != nil {
		log.Panic("Error:", err)
	}

	if gl.Init() != 0 {
		log.Panic("gl error")
	}

	if *vsync {
		glfw.SetSwapInterval(1)
	} else {
		glfw.SetSwapInterval(0)
	}

	glfw.SetWindowTitle(title)
	glfw.SetWindowSizeCallback(Reshape)

	Init()

	return func() {
		glfw.Terminate()
		glfw.CloseWindow()
		log.Print("Cleanup")
	}
}

func Reshape(width, height int) {
	if DoneThisFrame(ReshapeWindow) {
		return
	}

	gl.Viewport(0, 0, width, height)

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(-2.1, 6.1, -2.25, 2.1, -1, 1)
	//gl.Ortho(-2.1, 6.1, -2.25*2, 2.1*2, -1, 1) // Y debug

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	if Draw != nil {
		gl.DrawBuffer(gl.FRONT_AND_BACK)
		Draw()
		gl.DrawBuffer(gl.BACK)
	}
}
