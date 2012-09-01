package main

import (
	
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
	
	"github.com/banthar/gl"
	"github.com/banthar/glu"
	"github.com/jteeuwen/glfw"
)

var nrec = flag.Int64("nrec", 0, "number of records to read")

var nfram = flag.Int64("nfram", 100, "number of records to jump per frame")
var nback = flag.Int64("nback", 8000, "number of records to show")

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var verbose = flag.Bool("verbose", false, "Verbose")
var debug = flag.Bool("debug", false, "Turn on debugging")
var vsync = flag.Bool("vsync", true, "set to false to disable vsync")

var pageboundaries = flag.Bool("pageboundaries", false, "pageboundaries")

// TODO: Move these onto the file
var MAGIC_IN_RECORD = flag.Bool("magic-in-record", false, "Records contain magic bytes")
var PAGE_SIZE = flag.Uint64("page-size", 4096, "page-size")

var hide_qp_fraction = flag.Uint("hide-qp-fraction", 0,
	"If nonzero, pages with 'accesses < busiest / hqf' are ignored")

var margin_factor = float32(1) //0.975)

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
	gl.Vertex2d(0,  0.8)
	gl.Vertex2d(-0.8, 0)
	gl.Vertex2d( 0.8, 0)
	gl.End()
	gl.PopMatrix()
	
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}

func main_loop(data *ProgramData) {
	start := time.Now()
	frames := 0

	// Frame counter
	go func() {
		for {
			time.Sleep(time.Second)
			if (*verbose) {
				log.Print("fps = ", float64(frames)/time.Since(start).Seconds())
			}
			start = time.Now()
			frames = 0
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
		
	var i int64 = -int64(*nback)
	
	text := MakeText(data.filename, 32)
	
	// Location of mouse in record space
	var rec, rec_actual int64 = 0, 0
	
	var stacktext []*Text
	
	var mousex, mousey, mousedownx, mousedowny int
	var mousepx, mousepy float64
	var lbutton bool
	
	
	glfw.SetMouseWheelCallback(func(pos int) {
		// return
		// TODO: make this work
		nback_prev := *nback
		if pos < 0 {
			*nback = 40*1024 << uint(-pos)
		} else {
			*nback = 40*1024 >> uint(pos)
		}
		log.Print("Mousewheel position: ", pos, " nback: ", *nback)
		
		if rec == 0 { return }
		
		// mouse cursor is at screen "rec_actual", and it should be after
		// the transformation
		
		// We need to adjust `i` to keep this value constant:
		// rec_actual == i + int64(constpart * float64(*nback))
		//   where constpart <- (-const + 2.) / 4.
		// (that way, the mouse is still pointing at the same place after scaling)
		
		constpart := float64(rec_actual - i) / float64(nback_prev)
		
		rec_actual_after := i + int64(constpart * float64(*nback))
		delta := rec_actual_after - rec_actual
		i -= delta
	})
	
 	MouseToProj := func(x, y int) (float64, float64) {
		
		var projmat, modelmat [16]float64
		var viewport [4]int32
		
		gl.GetDoublev(gl.PROJECTION_MATRIX, projmat[0:15])
		gl.PushMatrix()
		gl.LoadIdentity()
		gl.GetDoublev(gl.MODELVIEW_MATRIX, modelmat[0:15])
		gl.PopMatrix()
		
		gl.GetIntegerv(gl.VIEWPORT, viewport[0:3])
		// Need to convert so that y is at lower left
		y = int(viewport[3]) - y
		
		px, py, _ := glu.UnProject(float64(x), float64(y), 0,
			&modelmat, &projmat, &viewport)
	   
		return px, py
	}
	
	var updated_this_frame bool = false
	
	update_stack := func() {
		if updated_this_frame { return }
		updated_this_frame = true
		
		for j := range stacktext {
			stacktext[j].destroy()
		}
		stack := data.GetStackNames(rec_actual)
		stacktext = make([]*Text, len(stack))
		for j := range stack {
			stacktext[j] = MakeText(stack[j], 32)
		}
	}
	
	glfw.SetMouseButtonCallback(func(button, action int) {
		switch button {
		case glfw.Mouse1:
			switch action {
			case glfw.KeyPress:
				mousedownx, mousedowny = mousex, mousey
				lbutton = true
			case glfw.KeyRelease:
				lbutton = false
			}
		}
	})
	
	
	glfw.SetMousePosCallback(func(x, y int) {
		
		px, py := MouseToProj(x, y)
		// Record index
		rec = int64((py + 2) * float64(*nback) / 4. + 0.5)
		rec_actual = rec + i
		
		dpy := py - mousepy
		di := int64(-dpy * float64(*nback) / 4.)
		
		if lbutton {
			i += di
		}
		
		mousepx, mousepy = px, py
		mousex, mousey = x, y
		
		//log.Printf("Mouse motion: (%3d, %3d), (%f, %f), (%d, %d) dpy=%f di=%d",
			//x, y, px, py, rec, rec_actual, dpy, di)
		
		if rec_actual > 0 && rec_actual < data.nrecords {
			log.Print(data.records[rec_actual])
		}
		
		update_stack()
	})
	
	done := false
	for !done {
		updated_this_frame = false
		
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LineWidth(1)
		
		// TODO: Ability to modify *nfram and *nback at runtiem
		//		 (i.e, pause and zoom functionality)
		i += *nfram

		N := *nback
		wrapped := data.Draw(i, N)
		if wrapped {
			i = -int64(*nback)
		}
		
		
		gl.PushMatrix()
		gl.Translated(0, -2, 0)
		gl.Scaled(1, 4 / float64(*nback), 1)
		gl.Translated(0, float64(rec), 0)
		
		gl.PointSize(5)
		gl.Begin(gl.POINTS)
		gl.Color4f(1, 1, 1, 1)
		gl.Vertex3f(0, 0, 0)
		gl.End()
		gl.PopMatrix()
		
		// TODO: Move matrix hackery somewhere else
		gl.MatrixMode(gl.PROJECTION)
		gl.PushMatrix()
		gl.LoadIdentity()
		
		// TODO: Use orthographic window co-ordinate space for font consistency?
		gl.Ortho(0, 400, 0, 400, -1, 1)
		gl.Color4f(1, 1, 1, 1)
		gl.Enable(gl.TEXTURE_2D)
		text.Draw(0, 0)
		gl.Disable(gl.TEXTURE_2D)
		gl.PopMatrix()
		gl.MatrixMode(gl.MODELVIEW)
		
		
		
		// TODO: Move matrix hackery somewhere else
		gl.MatrixMode(gl.PROJECTION)
		gl.PushMatrix()
		gl.LoadIdentity()
		
		// TODO: Use orthographic window co-ordinate space for font consistency?
		gl.Ortho(0, 1280, 0, 768, -1, 1)
		gl.Color4f(1, 1, 1, 1)
		gl.Enable(gl.TEXTURE_2D)
		for text_idx := range stacktext {
			stacktext[text_idx].Draw(1280*0.5, 768 - 35 - text_idx*16) //100+text_idx*32)
		}
		gl.Disable(gl.TEXTURE_2D)
		gl.PopMatrix()
		gl.MatrixMode(gl.MODELVIEW)
		
		OpenGLSentinel()
		glfw.SwapBuffers()

		done = glfw.Key(glfw.KeyEsc) != 0 || glfw.WindowParam(glfw.Opened) == 0
		frames += 1
		// Check for ctrl-c
		select {
		case <-interrupt:
			done = true
		default:
		}
	}
}


func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("Wrong number of arguments, expected 1, got ", flag.NArg())
	}

	if *cpuprofile != "" {
		log.Print("Profiling to ", *cpuprofile)
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *verbose {
		log.Print("Startup")
		defer log.Print("Shutdown")
	}

	data := NewProgramData(flag.Arg(0))

	cleanup := make_window(400, 400, "Memory Accesses")
	defer cleanup()
	
	main_loop(data)
}
