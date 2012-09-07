package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"log"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/banthar/gl"
	"github.com/banthar/glu"
	"github.com/jteeuwen/glfw"
)

// Used as "defer OpenGLSentinel()()" checks the gl error code on entry and exit
func OpenGLSentinel() func() {
	check := func() {
		e := gl.GetError()
		if e != gl.NO_ERROR {
			log.Panic("Encountered GLError: ", e)
		}
	}
	check()
	return check
}

type Vertex struct{ x, y float32 }
type Color struct{ r, g, b, a uint8 }
type ColorVertex struct {
	Color
	Vertex
}
type ColorVertices []ColorVertex

func (vcs ColorVertices) Draw() {
	if len(vcs) < 1 {
		return
	}

	gl.PushClientAttrib(0xFFFFFFFF) //gl.CLIENT_ALL_ATTRIB_BITS)
	defer gl.PopClientAttrib()

	gl.InterleavedArrays(gl.C4UB_V2F, 0, unsafe.Pointer(&vcs[0]))
	gl.DrawArrays(gl.LINES, 0, len(vcs))
}

func (vcs ColorVertices) DrawPartial(i, N int64) {
	if len(vcs) < 1 {
		return
	}
	//if i+N > int64(len(vcs)) { N = int64(len(vcs)) - i }
	if i+N > int64(len(vcs)) {
		i = int64(len(vcs)) - N
	}
	if i < 0 {
		i = 0
	}
	if N < 1 {
		return
	}
	//log.Print("N = ", N)

	if i+N > int64(len(vcs)) {
		N = int64(len(vcs)) - i
	}

	gl.PushClientAttrib(0xFFFFFFFF) //gl.CLIENT_ALL_ATTRIB_BITS)
	defer gl.PopClientAttrib()

	gl.InterleavedArrays(gl.C4UB_V2F, 0, unsafe.Pointer(&vcs[0]))
	OpenGLSentinel()
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

func GetViewportWH() (float64, float64) {
	var viewport [4]int32
	gl.GetIntegerv(gl.VIEWPORT, viewport[0:3])
	return float64(viewport[2]), float64(viewport[3])
}

func MouseToProj(x, y int) (float64, float64) {
	var projmat, modelmat [16]float64
	var viewport [4]int32

	gl.GetDoublev(gl.PROJECTION_MATRIX, projmat[0:15])
	gl.GetDoublev(gl.MODELVIEW_MATRIX, modelmat[0:15])

	gl.GetIntegerv(gl.VIEWPORT, viewport[0:3])
	// Need to convert so that y is at lower left
	y = int(viewport[3]) - y

	px, py, _ := glu.UnProject(float64(x), float64(y), 0,
		&modelmat, &projmat, &viewport)

	return px, py
}

func Reshape(width, height int) {
	gl.Viewport(0, 0, width, height)

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(-2.1, 6.1, -2.25, 2.1, -1, 1)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	if Draw != nil {
		Draw()
	}
}

type TextureBackedFBO struct {
	w, h    int
	texture gl.Texture
	gl.Framebuffer
	rbo gl.Renderbuffer
}

func NewTextureBackedFBO(w, h int) *TextureBackedFBO {
	fbo := &TextureBackedFBO{w, h, gl.GenTexture(), gl.GenFramebuffer(), gl.GenRenderbuffer()}

	With(Texture{fbo.texture, gl.TEXTURE_2D}, func() {
		gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		//gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
		//gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP)
		//gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP)
		gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		//gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
		// automatic mipmap generation included in OpenGL v1.4
		//gl.TexParameteri(gl.TEXTURE_2D, gl.GENERATE_MIPMAP, gl.TRUE)
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Bounds(), image.NewUniform(color.RGBA{128, 128, 0, 255}), image.ZP, draw.Src)
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, rgba.Pix)

		fd, err := os.Create("test.png")
		if err != nil {
			log.Panic("Err: ", err)
		}
		defer fd.Close()

		png.Encode(fd, rgba)
	})
	//return fbo

	With(Framebuffer{fbo}, func() {
		fbo.rbo.Bind()
		gl.RenderbufferStorage(gl.RENDERBUFFER, gl.DEPTH_COMPONENT, w, h)
		fbo.rbo.Unbind()

		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, fbo.texture, 0)

		fbo.rbo.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER)

		s := gl.CheckFramebufferStatus(gl.FRAMEBUFFER)
		if s != gl.FRAMEBUFFER_COMPLETE {
			log.Panic("Framebuffer issue: ", s)
		}
	})
	return fbo
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
	if valstat != 1 {
		log.Panic("Program validation failed: ", valstat)
	}

	return prog
}

func debug_coords() {
	// TODO: Move matrix hackery somewhere else
	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	//gl.LoadIdentity()
	//gl.Ortho(-2.1, 6.1, -4, 8, 1, -1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	gl.LoadIdentity()
	gl.LineWidth(5)
	gl.Color4f(1, 1, 0, 1)
	gl.Begin(gl.LINES)
	gl.Vertex2d(0, -1.6)
	gl.Vertex2d(0, 0.8)
	gl.Vertex2d(-0.8, 0)
	gl.Vertex2d(0.8, 0)
	gl.End()
	gl.PopMatrix()

	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}

func DrawQuadi(x, y, w, h int) {
	var u, v, u2, v2 float32 = 0, 1, 1, 0

	With(Primitive{gl.QUADS}, func() {
		gl.TexCoord2f(u, v)
		gl.Vertex2i(x, y)

		gl.TexCoord2f(u2, v)
		gl.Vertex2i(x+w, y)

		gl.TexCoord2f(u2, v2)
		gl.Vertex2i(x+w, y+h)

		gl.TexCoord2f(u, v2)
		gl.Vertex2i(x, y+h)
	})
}

func DrawQuadd(x, y, w, h float64) {
	var u, v, u2, v2 float32 = 0, 1, 1, 0

	With(Primitive{gl.QUADS}, func() {
		gl.TexCoord2f(u, v)
		gl.Vertex2d(x, y)

		gl.TexCoord2f(u2, v)
		gl.Vertex2d(x+w, y)

		gl.TexCoord2f(u2, v2)
		gl.Vertex2d(x+w, y+h)

		gl.TexCoord2f(u, v2)
		gl.Vertex2d(x, y+h)
	})
}
