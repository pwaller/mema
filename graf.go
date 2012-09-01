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
		N = int64(len(vcs)) - i
	}
	
	gl.PushClientAttrib(0xFFFFFFFF) //gl.CLIENT_ALL_ATTRIB_BITS)
	defer gl.PopClientAttrib()
	
	gl.InterleavedArrays(gl.C4UB_V2F, 0, unsafe.Pointer(&vcs[0]))
	OpenGLSentinel()
	gl.PointSize(2)
	gl.DrawArrays(gl.POINTS, int(i), int(N))
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

func MakeShader(shader_type gl.GLenum, source string) gl.Shader {
	
	shader := gl.CreateShader(shader_type)
	shader.Source(source)
	shader.Compile()
	OpenGLSentinel()
	
	compstat := shader.Get(gl.COMPILE_STATUS)
	if compstat != 1 {
		log.Print("vert shader compilation status: ", compstat)
		log.Print("Info log: ", shader.GetInfoLog())
		log.Panic("Problem creating shader?")
	}
	return shader
}

func MakeProgram() gl.Program {
	vert_shader := MakeShader(gl.VERTEX_SHADER, `
		#version 120
		// 330 compatibility

		//layout (location = 0) in vec4 position;
		//layout (location = 1) in vec4 color;

		varying out vec4 vertexcolor;
		//smooth out vec4 vertexcolor;

		// Passthrough vertex shader
		void main() {
			gl_Position = ftransform();
			gl_FrontColor = gl_Color;
		}
	`)
	
	// TODO: work in environments where geometry shaders are not available
	geom_shader := MakeShader(gl.GEOMETRY_SHADER, `
		#version 400
		#extension GL_EXT_geometry_shader4 : enable

		layout (points) in;
		layout (line_strip, max_vertices = 4) out;

		// This shader takes points and turns them into lines
		void main () {
			for(int i = 0; i < gl_VerticesIn; i++) {
				gl_Position = gl_in[i].gl_Position;
				gl_FrontColor =  gl_in[i].gl_FrontColor;
				EmitVertex();

				gl_Position = gl_in[i].gl_Position + vec4(0.0, 0.005, 0, 0);
				gl_FrontColor =  gl_in[i].gl_FrontColor;
				EmitVertex();
			}
		}
	`)
	
	frag_shader := MakeShader(gl.FRAGMENT_SHADER, `
		#version 120
		// 440 compatibility
		
		//in vec4 vertexcolor;
		//varying out vec4 outputColor;
		
		void main() {
			//gl_FragColor = gl_Color; //vertexcolor;
			//outputColor = vertexcolor;
			gl_FragColor = gl_Color;
		}
	`)
	
	prog := gl.CreateProgram()
	prog.AttachShader(vert_shader)
	prog.AttachShader(geom_shader)
	prog.AttachShader(frag_shader)
	prog.Link()
	
	OpenGLSentinel()
	
	// Note: These functions aren't implemented in master of banathar/gl
	// prog.ParameterEXT(gl.GEOMETRY_INPUT_TYPE_EXT, gl.POINTS)
	// prog.ParameterEXT(gl.GEOMETRY_OUTPUT_TYPE_EXT, gl.POINTS)
	// prog.ParameterEXT(gl.GEOMETRY_VERTICES_OUT_EXT, 4)
	
	// OpenGLSentinel()
	
	linkstat := prog.Get(gl.LINK_STATUS)
	if linkstat != 1 {
		log.Panic("Program link failed, status=", linkstat,
				  "Info log: ", prog.GetInfoLog())
	}
	
	prog.Validate()
	valstat := prog.Get(gl.VALIDATE_STATUS)
	if valstat != 1 { log.Panic("Program validation failed: ", valstat) }
	
	return prog
}
