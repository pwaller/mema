package main

import (
	"log"
	"runtime"
	"unsafe"
	
	"github.com/jteeuwen/glfw"
	"github.com/banthar/gl"
)


// Used as "defer OpenGLSentinel()()" checks the gl error code on entry and exit
func OpenGLSentinel() (func ()) {
	check := func () {
		e := gl.GetError()
		if e != gl.NO_ERROR {
			log.Panic("Encountered GLError: ", e)
		}
	}
	check()
	return check
}

type Vertex struct { x, y float32 }
type Color struct { r, g, b, a uint8 }
type ColorVertex struct { Color; Vertex }
type ColorVertices []ColorVertex

func (vcs ColorVertices) Draw() {
	if len(vcs) < 1 { return }
	
	gl.PushClientAttrib(0xFFFFFFFF) //gl.CLIENT_ALL_ATTRIB_BITS)
	defer gl.PopClientAttrib()
	
	gl.InterleavedArrays(gl.C4UB_V2F, 0, unsafe.Pointer(&vcs[0]))
	gl.DrawArrays(gl.LINES, 0, len(vcs))
}

func (vcs ColorVertices) DrawPartial(i, N int64) {
	if len(vcs) < 1 { return }
	//if i+N > int64(len(vcs)) { N = int64(len(vcs)) - i }
	if i+N > int64(len(vcs)) { i = int64(len(vcs)) - N }
	if i < 0 { i = 0 }
	if N < 1 { return }
	//log.Print("N = ", N)
	
	
	if i+N > int64(len(vcs)) {
		log.Panic("Too many: ", N, " ", i+N, " ", len(vcs))
	}
	
	gl.PushClientAttrib(0xFFFFFFFF) //gl.CLIENT_ALL_ATTRIB_BITS)
	defer gl.PopClientAttrib()
	
	gl.InterleavedArrays(gl.C4UB_V2F, 0, unsafe.Pointer(&vcs[0]))
	OpenGLSentinel()
	gl.DrawArrays(gl.LINES, int(i), int(N))
	defer func() {
		if r := recover(); r != nil {
			log.Print("i = ", i, " N = ", N)
			log.Panic(r)
		}
	}()
	OpenGLSentinel()
}

func (vcs *ColorVertices) Add(v ColorVertex) {
	*vcs = append(*vcs, v)
}

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
	gl.Ortho(-2.1, 2.1, -2.25, 2.1, -1, 1)
	
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
	// TODO: For smoothness we must immediately redraw here
	//gl.Translatef(0.0, 0.0, -40.0)
}
