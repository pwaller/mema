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
	
	text := MakeText("Hello, world", 32.)
	// Location of mouse in record space
	var rec, rec_actual int64 = 0, 0
	
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
		var view [4]int32
		
		gl.GetDoublev(gl.PROJECTION_MATRIX, projmat[0:15])
		gl.PushMatrix()
		gl.LoadIdentity()
		gl.GetDoublev(gl.MODELVIEW_MATRIX, modelmat[0:15])
		gl.PopMatrix()
		
		gl.GetIntegerv(gl.VIEWPORT, view[0:3])
		
		px, py, _ := glu.UnProject(float64(x), float64(y), 0,
					   &modelmat, &projmat, &view)
	   
		return px, py
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
		rec = int64((-py + 2.) * float64(*nback) / 4.)
		rec_actual = rec + i
		
		dpy := py - mousepy
		di := int64(dpy * float64(*nback) / 4.)
		
		if lbutton {
			i += di
		}
		
		mousepx, mousepy = px, py
		mousex, mousey = x, y
		
		//log.Printf("Mouse motion: (%3d, %3d), (%f, %f), (%d, %d) dpy=%f di=%f", x, y, px, py, rec, rec_actual, dpy, di)
		
		if rec_actual > 0 && rec_actual < data.nrecords {
			log.Print(data.records[rec_actual])
		}
		
	})
	
	done := false
	for !done {
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
