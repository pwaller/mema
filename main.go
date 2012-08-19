package main

import (
	
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
	
	"github.com/banthar/gl"
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
	
	glfw.SetMouseWheelCallback(func(pos int) {
		// return
		// TODO: make this work
		if pos > 0 {
			*nback = 40*1024 << uint(pos)
		} else {
			*nback = 40*1024 >> uint(-pos)
		}
		log.Print("Mousewheel position: ", pos, " nback: ", *nback)
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
