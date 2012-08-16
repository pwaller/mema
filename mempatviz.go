package main

// TODO: Load on demand sections which will be read
// TODO: pre-compute what needs to be rendered

// python file position at startup: 6649665347

// end of building 0 containing array: 6657004931
// end of "from random import random" 7424515715 / 7425564227
// end of building random array: 7436049347
// end of sorting random array: 7442340419
// final size: 7752248915

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


var margin_factor = float32(1) //0.975)

func main_loop(data ProgramData) {
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
	
	done := false
	for !done {
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LineWidth(1)

		i += *nfram

		gl.Begin(gl.LINES)
		N := *nback
		wrapped := data.Draw(i, N)
		if wrapped {
			i = -int64(*nback)
		}
		gl.End()

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
