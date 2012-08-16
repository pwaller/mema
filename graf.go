package main

import (
    "log"
    "runtime"
    
    "github.com/jteeuwen/glfw"
    "github.com/banthar/gl"
)

func Init() {
	//gl.Enable(gl.DEPTH_TEST)
	
	// Anti-aliasing
    //gl.Enable(gl.LINE_SMOOTH)
    gl.Enable(gl.BLEND)
    //gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
    gl.BlendFunc(gl.SRC_ALPHA, gl.DST_ALPHA)
    //gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.ONE)
    //gl.Hint(gl.LINE_SMOOTH_HINT, gl.NICEST)
}

func make_window(w, h int, title string) func() {
    // Required to make sure that the OpenGL go-routine doesn't get switched
    // to another thread (=> kerblammo)
    runtime.LockOSThread()

	if err := glfw.Init(); err != nil { log.Panic("glfw Error:", err) }
		
    err := glfw.OpenWindow(w, h, 0, 0, 0, 0, 0, 0, glfw.Windowed)
	if err != nil { log.Panic("Error:", err) }
	
	if gl.Init() != 0 { log.Panic("gl error") }

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
	//h := float64(height) / float64(width)

	gl.Viewport(0, 0, width, height)
	
	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()	
	//gl.Frustum(-1.0, 1.0, -h, h, 5.0, 60.0)
	gl.Ortho(-2, 2, -2, 2.1, -1, 1)
	
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	// TODO: For smoothness we must immediately redraw here
	//gl.Translatef(0.0, 0.0, -40.0)
}
