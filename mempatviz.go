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
	"image"
	"image/png"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"time"
	
	"github.com/banthar/gl"
	"github.com/jteeuwen/glfw"
)

var printInfo = flag.Bool("info", false, "print GL implementation information")

var jump = flag.Int64("jump", 0, "jump to file offset")
var nrec = flag.Int64("nrec", 0, "number of records to read")

var nfram = flag.Int64("nfram", 100, "number of records to jump per frame")
var nback = flag.Int64("nback", 8000, "number of records to show")

var verbose = flag.Bool("verbose", false, "Verbose")
var pageboundaries = flag.Bool("pageboundaries", false, "pageboundaries")

// TODO: Move these onto the file
var compressed = flag.Bool("compressed", true, "input file uses compression")
var MAGIC_IN_RECORD = flag.Bool("magic-in-record", false, "Records contain magic bytes")
var PAGE_SIZE = flag.Uint64("page-size", 4096, "page-size")

var region = flag.Int("region", 0, "Region index to view")

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

var vsync = flag.Bool("vsync", true, "set to false to disable vsync")

var margin_factor = float32(0.95)


func draw() {

}

func Capture() {
	// TODO: co-ordinates, filename, cleverness to stitch many together
	im := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	gl.ReadBuffer(gl.BACK_LEFT)
	gl.ReadPixels(0, 0, 400, 400, gl.RGBA, im.Pix)
	
	fd, err := os.Create("test.png")
	if err != nil { log.Panic("Err: ", err) }
	defer fd.Close()
	
	png.Encode(fd, im)
}

func main_loop(data ProgramData) {
	
	last_frame_epoch := time.Now()
	elapsed := time.Duration(0)
	start := time.Now()

	frames := 0

	// Frame counter
	go func() {
		//return
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

	log.Print("Active Region IDs:")
	active_regions := data.ActiveRegionIDs()
	for i := range active_regions {
		log.Print(active_regions[i])
	}
	
	
	done := false
	active_region := *region
	var i int64 = -int64(*nback) //len(data.access))
	log.Print("Active Region: ", data.region[active_region])
	for !done {
		draw()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LineWidth(1)

		i += *nfram

		gl.Begin(gl.LINES)
		N := *nback
		wrapped := data.Draw(i, N, active_regions[active_region])
		if wrapped {
			i = -int64(*nback)
			active_region = (active_region + 1) % len(active_regions)
			log.Print("Active Region: ", data.region[active_region])
		}
		gl.End()

		glfw.SwapBuffers()

		_ = elapsed

		done = glfw.Key(glfw.KeyEsc) != 0 || glfw.WindowParam(glfw.Opened) == 0
		frames += 1
		// Check for ctrl-c
		select {
		case <-interrupt:
			done = true
		default:
		}

		elapsed = time.Since(last_frame_epoch)
		last_frame_epoch = time.Now()
		continue
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

	data := parse_file(flag.Arg(0))

	cleanup := make_window(400, 400, "Memory Accesses")
	defer cleanup()

	main_loop(data)
}
