package main

// python file position at startup: 6649665347

// end of building 0 containing array: 6657004931
// end of "from random import random" 7424515715 / 7425564227
// end of building random array: 7436049347
// end of sorting random array: 7442340419
// final size: 7752248915

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/banthar/gl"
	"github.com/jteeuwen/glfw"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"
)

var printInfo = flag.Bool("info", false, "print GL implementation information")

var jump = flag.Int64("jump", 0, "jump to file offset")
var nrec = flag.Int64("nrec", 0, "number of records to read")

var nfram = flag.Int64("nfram", 100, "number of records to jump per frame")
var nback = flag.Int64("nback", 8000, "number of records to show")

type MemAccess struct {
	Time             float64
	Pc, Bp, Sp, Addr uint64
	IsWrite          uint64 // because alignment.
}

func (a MemAccess) String() string {
	return fmt.Sprintf("MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
		a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
}

func draw() {

}

func main_loop(target_fps int, records []MemAccess) {

	target_frame_period := time.Second / time.Duration(target_fps)
	log.Print("target_frame_period = ", target_frame_period)

	last_frame_epoch := time.Now()
	elapsed := time.Duration(0)
	start := time.Now()

	frames := 0

	// Frame counter
	go func() {
		//return
		for {
			time.Sleep(time.Second)
			log.Print("fps = ", float64(frames)/time.Since(start).Seconds())
			start = time.Now()
			frames = 0
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	var minAddr, maxAddr uint64 = math.MaxUint64, 0

	for i := range records { //}:= 0; i < 100; i++ {
		//log.Print(records[i])
		r := &records[i]
		if r.Addr < 0x7fff00000000 {
			continue
		}
		if r.Addr < minAddr {
			minAddr = r.Addr
		}
		if r.Addr > maxAddr {
			maxAddr = r.Addr
		}
	}

	log.Printf("Min = 0x%x - 0x%x", minAddr, maxAddr)
	widthAddr := maxAddr - minAddr

	var i int64 = 0

	done := false
	for !done {
		draw()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LineWidth(0.5)

		i += *nfram
		//if i >= len(records) { i = 0 }

		gl.Begin(gl.LINES)
		N := *nback
		for j := int64(0); j < N; j++ {
			pos := (i + j) % int64(len(records))
			r := records[pos]

			x := float32(r.Addr - minAddr) / float32(widthAddr)
			x = (x - 0.5) * 4
			y := -2 + 4*float32(j) / float32(N)

			if pos == 0 {
				//log.Print("Pos = 0 y =", y)
				gl.End()
				gl.LineWidth(10)
				gl.Begin(gl.LINES)
				gl.Color4f(1, 1, 1, 1)
				gl.Vertex3f(-10, y, -10)
				gl.Vertex3f(10, y, -10)
				gl.End()
				gl.LineWidth(0.5)
				gl.Begin(gl.LINES)
				*nfram = 100
			}

			if r.Addr < 0x7fff00000000 {
				continue
			}
			gl.Color4d(float64(1-r.IsWrite), float64(r.IsWrite), 0, 1+math.Log(1.-float64(N-j)/float64(N))/3)
			gl.Vertex3f(x, y, -10)
			gl.Vertex3f(x, y+0.05, -10)
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

		remaining_time := target_frame_period - time.Since(last_frame_epoch)
		_ = remaining_time
		time.Sleep(remaining_time)

		elapsed = time.Since(last_frame_epoch)
		last_frame_epoch = time.Now()
		continue
	}
}

func parse_file(filename string) []MemAccess {
	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		log.Panic("Fatal error: ", err)
	}

	reader := bufio.NewReader(fd)
	page_table_bytes, err := reader.ReadBytes('\x00')
	if err != nil {
		log.Panic("Error reading file: ", err)
	}

	page_table := string(page_table_bytes[:len(page_table_bytes)-1])
	page_table_lines := strings.Split(page_table, "\n")

	for i := range page_table_lines {
		line := page_table_lines[i]
		if len(line) == 0 {
			continue
		}
		var low, hi uint64
		var what string

		_, err := fmt.Sscanf(line, " 0x%x-0x%x %s", &low, &hi, &what)
		if err != nil {
			_, err := fmt.Sscanf(line, " 0x%x-0x%x", &low, &hi)
			what = "<unk>"
			if err != nil {
				log.Panic("Error parsing: ", err, " '", line, "'")
			}
		}
	}

	if *jump != 0 {
		fd.Seek(*jump, os.SEEK_SET)
		reader = bufio.NewReader(fd)
	}

	var records []MemAccess

	for {
		x := MemAccess{}
		err = binary.Read(reader, binary.LittleEndian, &x)
		if err != nil {
			break
		}
		records = append(records, x)
		if *nrec != 0 && int64(len(records)) > *nrec {
			break
		}
	}

	log.Print("Read ", len(records))

	return records
}

func main() {

	log.Print("Startup")
	defer log.Print("Shutdown")

	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("Wrong number of arguments, expected 1, got ", flag.NArg())
	}

	records := parse_file(flag.Arg(0))

	cleanup := make_window(400, 400, "Memory Accesses")
	defer cleanup()

	target_fps := 60

	main_loop(target_fps, records)
}
