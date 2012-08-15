package main

import (
    "log"
    "runtime"
    
    "github.com/jteeuwen/glfw"
    "github.com/banthar/gl"
)

func Init(printInfo bool) {
	//pos := []float32{5.0, 5.0, 10.0, 0.0}
	//red := []float32{0.8, 0.1, 0.0, 1.0}
	//green := []float32{0.0, 0.8, 0.2, 1.0}
	//blue := []float32{0.2, 0.2, 1.0, 1.0}

	//gl.Lightfv(gl.LIGHT0, gl.POSITION, pos)
	
	//gl.Enable(gl.CULL_FACE)
	//gl.Enable(gl.LIGHTING)
	gl.Enable(gl.LIGHT0)
	//gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.NORMALIZE)
	
	// Anti-aliasing
    gl.Enable(gl.LINE_SMOOTH)
    gl.Enable(gl.BLEND)
    //gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
    gl.BlendFunc(gl.SRC_ALPHA, gl.DST_ALPHA)
    //gl.BlendFunc(gl.SRC_ALPHA_SATURATE, gl.ONE)
    gl.Hint(gl.LINE_SMOOTH_HINT, gl.NICEST)
    gl.LineWidth(1.5) //0.5)
    
	if printInfo {
		print("GL_RENDERER   = ", gl.GetString(gl.RENDERER), "\n")
		print("GL_VERSION    = ", gl.GetString(gl.VERSION), "\n")
		print("GL_VENDOR     = ", gl.GetString(gl.VENDOR), "\n")
		print("GL_EXTENSIONS = ", gl.GetString(gl.EXTENSIONS), "\n")
	}

}

func make_window(w, h int, title string) func() {
    // Required to make sure that the OpenGL go-routine doesn't get switched
    // to another thread (=> kerblammo)
    runtime.LockOSThread()

	if err := glfw.Init(); err != nil { log.Panic("glfw Error:", err) }
	
    err := glfw.OpenWindow(w, h, 0, 0, 0, 0, 0, 0, glfw.Windowed)
	if err != nil { log.Panic("Error:", err) }
	
	if gl.Init() != 0 { log.Panic("gl error") }

	glfw.SetWindowTitle(title)
	glfw.SetWindowSizeCallback(Reshape)

	Init(*printInfo)
	
	return func() {
	    glfw.Terminate()
	    glfw.CloseWindow()
	    log.Print("Cleanup")
    }
}

func Reshape(width, height int) {
	h := float64(height) / float64(width)

	gl.Viewport(0, 0, width, height)
	
	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()	
	gl.Frustum(-1.0, 1.0, -h, h, 5.0, 60.0)
	
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	//gl.Translatef(0.0, 0.0, -40.0)
}
