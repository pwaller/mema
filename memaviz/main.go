package main

import (
	//"debug/dwarf"

	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/go-gl/gl"
	"github.com/go-gl/glfw"
	"github.com/go-gl/glh"

	"net/http"
	_ "net/http/pprof"
)

var nback = flag.Int64("nback", 8000, "number of records to show")

var profiling = flag.Bool("prof", false, "enable profiling")
var memprofile = flag.String("memprofile", "", "write mem profile to file")

var verbose = flag.Bool("verbose", false, "Verbose")
var debug = flag.Bool("debug", false, "Turn on debugging")
var vsync = flag.Bool("vsync", true, "set to false to disable vsync")

var use_stree = flag.Bool("stree", false, "Enable stree (may cause GC problems)")

var pageboundaries = flag.Bool("pageboundaries", false, "pageboundaries")

// TODO: Move these onto the file
var MAGIC_IN_RECORD = flag.Bool("magic-in-record", false, "Records contain magic bytes")
var PAGE_SIZE = flag.Uint64("page-size", 4096, "page-size")

var hide_qp_fraction = flag.Uint("hide-qp-fraction", 0,
	"If nonzero, pages with 'accesses < busiest / hqf' are ignored")

var margin_factor = float32(1) //0.975)

// Redraw
var Draw func() = nil

// 10 = it's possible to schedule 100 actions per frame
var main_thread_work chan func() = make(chan func(), 10)

func DoMainThreadWork() {
	// Run all work scheduled for the main thread
	start := time.Now()
	for have_work := true; have_work && time.Since(start) < 5*time.Millisecond; {
		select {
		case f := <-main_thread_work:
			f()
		default:
			have_work = false
		}
	}
}

type WorkType int

const (
	RenderText WorkType = iota
	ReshapeWindow
)

var done_this_frame map[WorkType]bool = make(map[WorkType]bool)

func DoneThisFrame(value WorkType) bool {
	_, present := done_this_frame[value]
	if !present {
		done_this_frame[value] = true
	}
	return present
}

func main_loop(data *ProgramData) {
	start := time.Now()
	frames := 0
	lastblocks := 0

	// Frame counter
	go func() {
		for {
			time.Sleep(time.Second)
			if *verbose {
				memstats := new(runtime.MemStats)
				runtime.ReadMemStats(memstats)
				fps := float64(frames) / time.Since(start).Seconds()
				blocks := len(data.blocks)
				bps := float64(blocks-lastblocks) / time.Since(start).Seconds()
				lastblocks = blocks
				log.Printf("fps = %5.2f; blocks = %4d; bps = %5.2f sparemem = %6d MB; alloc'd = %6.6f; (+footprint = %6.6f)",
					fps, len(data.blocks), bps, SpareRAM(), float64(memstats.Alloc)/1024/1024,
					float64(memstats.Sys-memstats.Alloc)/1024/1024)

				PrintTimers(frames)
			}
			start = time.Now()
			frames = 0
		}
	}()

	// Necessary at the moment to prevent eventual OOM
	go func() {
		for {
			time.Sleep(5 * time.Second)
			if *verbose {
				log.Print("GC()")
			}
			runtime.GC()
			if *verbose {
				GCStats()
			}
		}
	}()

	var i int64 = -int64(*nback)

	text := glh.MakeText(data.filename, 32)

	// Location of mouse in record space
	var rec, rec_actual int64 = 0, 0

	var stacktext, dwarftext []*glh.Text
	var recordtext *glh.Text = nil

	var mousex, mousey, mousedownx, mousedowny int
	var mousepx, mousepy float64
	var lbutton bool
	escape_hit := false

	glfw.SetMouseWheelCallback(func(pos int) {
		nback_prev := *nback
		if pos < 0 {
			*nback = 40 * 1024 << uint(-pos)
		} else {
			*nback = 40 * 1024 >> uint(pos)
		}
		//log.Print("Mousewheel position: ", pos, " nback: ", *nback)

		if rec == 0 {
			return
		}

		// mouse cursor is at screen "rec_actual", and it should be after
		// the transformation

		// We need to adjust `i` to keep this value constant:
		// rec_actual == i + int64(constpart * float64(*nback))
		//   where constpart <- (-const + 2.) / 4.
		// (that way, the mouse is still pointing at the same place after scaling)

		constpart := float64(rec_actual-i) / float64(nback_prev)

		rec_actual_after := i + int64(constpart*float64(*nback))
		delta := rec_actual_after - rec_actual
		i -= delta

		// Ensure the mouse cursor position doesn't change when zooming
		rec = rec_actual - i
	})

	update_text := func() {
		if DoneThisFrame(RenderText) {
			return
		}

		if recordtext != nil {
			recordtext.Destroy()
			recordtext = nil
		}
		//r := 0 //data.GetRecord(rec_actual)
		//if r != nil {
		//log.Print(data.records[rec_actual])
		//recordtext = MakeText(r.String(), 32)
		//}

		for j := range stacktext {
			stacktext[j].Destroy()
		}
		// TODO: Load records on demand
		if false {
			stack := data.GetStackNames(rec_actual)
			stacktext = make([]*glh.Text, len(stack))
			for j := range stack {
				stacktext[j] = glh.MakeText(stack[j], 32)
			}
		}
	}

	glfw.SetMouseButtonCallback(func(button, action int) {
		switch button {
		case glfw.Mouse1:
			switch action {
			case glfw.KeyPress:
				mousedownx, mousedowny = mousex, mousey
				lbutton = true

				r := data.GetRecord(rec_actual)
				if r != nil {
					if r.Type == MEMA_ACCESS {
						log.Print(r)
						ma := r.MemAccess()
						dwarf := data.GetDwarf(ma.Pc)
						log.Print("Can has dwarf? ", len(dwarf))
						for i := range dwarf {
							log.Print("  ", dwarf[i])
							//recordtext = MakeText(data.records[rec_actual].String(), 32)

						}
						log.Print("")

						for j := range dwarftext {
							dwarftext[j].Destroy()
						}
						dwarftext = make([]*glh.Text, len(dwarf))
						for j := range dwarf {
							dwarftext[j] = glh.MakeText(fmt.Sprintf("%q", dwarf[j]), 32)
						}
					}

					//recordtext = MakeText(data.records[rec_actual].String(), 32)
				}

			case glfw.KeyRelease:
				lbutton = false
			}
		}
	})

	glfw.SetMousePosCallback(func(x, y int) {

		px, py := glh.WindowToProj(x, y)
		// Record index
		rec = int64((py+2)*float64(*nback)/4. + 0.5)
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

		update_text()
	})

	glfw.SetKeyCallback(func(key, state int) {
		switch key {
		case glfw.KeyEsc:
			escape_hit = true
		}
	})

	draw_mousepoint := func() {

		// Draw the mouse point
		glh.With(glh.Matrix{gl.MODELVIEW}, func() {
			gl.Translated(0, -2, 0)
			gl.Scaled(1, 4/float64(*nback), 1)
			gl.Translated(0, float64(rec), 0)

			gl.PointSize(10)
			glh.With(glh.Primitive{gl.POINTS}, func() {
				gl.Color4f(1, 1, 1, 1)
				gl.Vertex3d(mousepx, 0, 0)
			})
		})
	}

	draw_text := func() {
		// Draw any text
		glh.With(glh.WindowCoords{}, func() {
			w, h := glh.GetViewportWHD()

			glh.With(glh.Attrib{gl.ENABLE_BIT}, func() {
				gl.Enable(gl.TEXTURE_2D)
				text.Draw(0, 0)
				for text_idx := range stacktext {
					stacktext[text_idx].Draw(int(w*0.55), int(h)-35-text_idx*16)
				}
				for text_idx := range dwarftext {
					dwarftext[text_idx].Draw(int(w*0.55), 35+text_idx*16)
				}
				if recordtext != nil {
					recordtext.Draw(int(w*0.55), 35)
				}
			})
		})
	}

	Draw = func() {

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// Draw the memory access/function data
		data.Draw(i, *nback)

		draw_mousepoint()
		draw_text()

		// Visible region quad
		// gl.Color4f(1, 1, 1, 0.25)
		// DrawQuadd(-2.1, -2.25, 4.4, 2.1 - -2.25)

		StatsHUD()
	}

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt)
	ctrlc_hit := false
	go func() {
		<-interrupt
		ctrlc_hit = true
	}()

	done := false
	for !done {
		done_this_frame = make(map[WorkType]bool)

		glh.With(&Timer{Name: "Draw"}, func() {
			Draw()
		})

		glfw.SwapBuffers()

		DoMainThreadWork()

		done = ctrlc_hit || escape_hit || glfw.WindowParam(glfw.Opened) == 0
		frames += 1
	}
}

func FailSafe() {
	for {
		time.Sleep(1 * time.Microsecond)
		if SystemFree() < 200e6 {
			DumpStatsHUD()
			log.Fatal("Less than 200MB system RAM free. Aborting")
		}
	}
}

func main() {
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lshortfile)

	go FailSafe()

	if *profiling {
		go func() { log.Println(http.ListenAndServe(":6060", nil)) }()
	}

	if *verbose {
		log.Print("Startup")
		defer log.Print("Shutdown")
	}

	var data *ProgramData
	var action = "visualize"

	switch flag.NArg() {
	default:
		flag.Usage()
		println()
		println("memaviz [action] filename.mema")
		println("  actions:")
		println("    pack")
		println()
		return
	case 1:
		data = NewProgramData(flag.Arg(0))

	case 2:
		action = flag.Arg(0)
		data = NewProgramData(flag.Arg(1))
	}

	switch action {
	case "visualize":
		cleanup := make_window(1280, 768, "Memory Accesses")
		defer cleanup()

		InitStatsHUD()
		main_loop(data)
	case "pack":
		data.PackBinaries()

	default:
		log.Fatal("Unknown action: %q", action)
	}
}
