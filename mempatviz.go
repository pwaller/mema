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
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"
	
	"github.com/banthar/gl"
	"github.com/jteeuwen/glfw"
	// "code.google.com/p/snappy-go/snappy"
	"github.com/mnuhn/go-lz4"
)

var printInfo = flag.Bool("info", false, "print GL implementation information")

var jump = flag.Int64("jump", 0, "jump to file offset")
var nrec = flag.Int64("nrec", 0, "number of records to read")

var nfram = flag.Int64("nfram", 100, "number of records to jump per frame")
var nback = flag.Int64("nback", 8000, "number of records to show")

var verbose = flag.Bool("verbose", false, "Verbose")
var pageboundaries = flag.Bool("pageboundaries", false, "pageboundaries")

var MAGIC_IN_RECORD = flag.Bool("magic-in-record", false, "Records contain magic bytes")

var margin_factor = float32(0.95)

type MemRegion struct {
	low, hi uint64
	perms, offset, dev, inode, pathname string
}

type MemAccess struct {
	Time             float64
	Pc, Bp, Sp, Addr uint64
	IsWrite          uint64 // because alignment.
}

type ProgramData struct {
	region []MemRegion
	access []MemAccess
}

func (a MemAccess) String() string {
	return fmt.Sprintf("MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
		a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
}

func (d ProgramData) RegionID(addr uint64) int {
	for i := range d.region {
		if d.region[i].low < addr && addr < d.region[i].hi { return i }
	}
	//log.Panicf("Address 0x%x not in any defined memory region!", addr)
	return len(d.region)
}

func (d ProgramData) ActiveRegionIDs() []int {
	active := make(map[int] bool)
	for i := range d.access {
		active[d.RegionID(d.access[i].Addr)] = true
	}
	result := make([]int, 0)
	for k, _ := range active {
		result = append(result, k)
	}
	sort.Ints(result)
	return result
}

func min(a, b int64) int64 { if b < a { return b }; return a }

func (data ProgramData) Draw(start, N int64, region_id int) bool {

	//region := data.region[region_id]

	var minAddr, maxAddr uint64 = math.MaxUint64, 0

	for pos := start; pos < min(start + N, int64(len(data.access))); pos++ {
		if pos < 0 { continue }
		r := data.access[pos]
		if data.RegionID(r.Addr) != region_id { continue }
		if r.Addr < minAddr {
			minAddr = 4096 * (r.Addr / 4096)
		}
		if r.Addr > maxAddr {
			maxAddr = 4096 * (r.Addr / 4096 + 1)
		}
	}

	width := maxAddr - minAddr
	if width == 0 { width = 1 }

	if *pageboundaries {
		if width / 4096 < 100 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p < width; p += 4096 {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				gl.Color4d(1, 1, 1, 0.25)
				x *= margin_factor
				gl.Vertex3f(x, -2, -10)
				gl.Vertex3f(x, 2, -10)
			}
		}
	}

	for pos := start; pos < min(start + N, int64(len(data.access))); pos++ {
		if pos < 0 { continue }
		r := data.access[pos]
		i := data.RegionID(r.Addr)
		if i != region_id { continue }
		x := float32(r.Addr - minAddr) / float32(width)
		x = (x - 0.5) * 4
		x *= margin_factor
		y := -2 + 4*float32(pos - start) / float32(N)
		gl.Color4d(float64(1-r.IsWrite), float64(r.IsWrite), 0, 1+math.Log(1.-float64(N - (pos - start))/float64(N))/3)
		gl.Vertex3f(x, y, -10)
		gl.Vertex3f(x, y+0.0125, -10)
		gl.Color4d(float64(1-r.IsWrite), float64(r.IsWrite), 0, 1)
		gl.Vertex3f(x, 2 + 0.1, -10)
		gl.Vertex3f(x, 2, -10)

	}
	if start + N > int64(len(data.access)) {
		y := -2 + 4*float32(int64(len(data.access)) - start) / float32(N)
		gl.Color4d(1, 1, 1, 1)
		gl.Vertex3f(-2, y, -10)
		gl.Vertex3f( 2, y, -10)
	}
	return start > int64(len(data.access))

}

func draw() {

}

func main_loop(target_fps int, data ProgramData, ) {

	target_frame_period := time.Second / time.Duration(target_fps)
	
	if (*verbose) {
		log.Print("target_frame_period = ", target_frame_period)
	}

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
	active_region := 0
	var i int64 = -int64(*nback) //len(data.access))
	for !done {
		draw()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LineWidth(0.5)

		i += *nfram
		//if i >= len(records) { i = 0 }

		gl.Begin(gl.LINES)
		N := *nback
		wrapped := data.Draw(i, N, active_regions[active_region])
		if wrapped {
			i = -int64(*nback)
			active_region = (active_region + 1) % len(active_regions)
			log.Print("Active Region: ", active_regions[active_region])
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

const (
	MEMA_ACCESS = 0
	MEMA_FUNC_ENTER = 1
	MEMA_FUNC_EXIT = 2
)

type Reader interface {
	Read([]byte) (int, error)
}

func parse_block(decompressed_reader Reader, data *ProgramData) {
	x := MemAccess{}
	var v int64

	//r := bufio.NewReader(decompressed_reader)
	//buf, err := r.Peek(8)
	//log.Printf("Bytes: %q err: %q", buf, err)

	i := 0
	buf := make([]byte, 48)
	//junk := make([]byte, 128 - 48)
	for {
		i++
		err := binary.Read(decompressed_reader, binary.LittleEndian, &v)
		if MAGIC_IN_RECORD {
			var magic uint64
			err = binary.Read(decompressed_reader, binary.LittleEndian, &magic)
			if magic != 0xDEADBEEF {
				log.Printf("  magic bytes: %x", magic)
			}
		}
		if v == MEMA_ACCESS {
			err = binary.Read(decompressed_reader, binary.LittleEndian, &x)
			//decompressed_reader.Read(junk)
			//log.Print("Value of v:", v, " x: ", x, " err:", err)
		} else if v == MEMA_FUNC_ENTER {
			// TODO: Use index into string map instead
			// TODO: Encode this somehow. Also deref the symbol from the symbol
			//       table

			n, err := io.ReadFull(decompressed_reader, buf)
			if err != nil || n != len(buf) {
				log.Panic("n = ", n, " err = ", err, )
			}
			func_addr := binary.LittleEndian.Uint64(buf)
			_ = func_addr
			//err := binary.Read(decompressed_reader, binary.LittleEndian, &func_addr)
			//log.Printf("Function Enter: 0x%x", func_addr)
		} else if v == MEMA_FUNC_EXIT {
			n, err := io.ReadFull(decompressed_reader, buf)
			if err != nil || n != len(buf) {
				log.Panic("n = ", n, " err = ", err)
			}
			func_addr := binary.LittleEndian.Uint64(buf)
			_ = func_addr
			//err := binary.Read(decompressed_reader, binary.LittleEndian, &func_addr)
			//log.Printf("Function Exit: 0x%x", func_addr)
		} else {
			log.Panic("Unknown event e = ", v, " err:", err)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Panic("Unexpected error: ", err)
		}
		data.access = append(data.access, x)
	}

}

func parse_file(filename string) ProgramData {
	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		log.Panic("Fatal error: ", err)
	}

	magic_buf := make([]byte, 8)

	_, err = fd.Read(magic_buf)
	if err != nil || string(magic_buf) != "MEMACCES" {
		log.Panic("Error reading magic bytes: ", err, " bytes=", magic_buf)
	}

	reader := bufio.NewReaderSize(fd, 10*1024*1024)
		
	page_table_bytes, err := reader.ReadBytes('\x00')
	if err != nil {
		log.Panic("Error reading file: ", err)
	}
	
	log.Print("Read ", len(page_table_bytes), " bytes")

	var data ProgramData

	page_table := string(page_table_bytes[:len(page_table_bytes)-1])
	page_table_lines := strings.Split(page_table, "\n")

	for i := range page_table_lines {
		line := page_table_lines[i]
		if len(line) == 0 {
			continue
		}
		x := MemRegion{}

		_, err := fmt.Sscanf(line, "%x-%x %s %s %s %s %s", &x.low, &x.hi, 
							 &x.perms, &x.offset, &x.dev, &x.inode, &x.pathname)
		if err != nil {
			_, err := fmt.Sscanf(line, "%x-%x %s %s %s %s", &x.low, &x.hi, 
								 &x.perms, &x.offset, &x.dev, &x.inode)
			x.pathname = ""
			if err != nil {
				log.Panic("Error parsing: ", err, " '", line, "'")
			}
		}
		data.region = append(data.region, x)
	}

	if *jump != 0 {
		fd.Seek(*jump, os.SEEK_SET)
		reader = bufio.NewReader(fd)
	}

	//reader = bufio.NewReader(fd)

	for {

		offset, err := fd.Seek(0, os.SEEK_CUR)
		log.Print("Reading from location ", offset-int64(reader.Buffered()), " buf = ", reader.Buffered(), " offs = ", offset)
		buf, err := reader.Peek(8)
		log.Printf("Bytes: %q", buf)

		var block_size int64
		err = binary.Read(reader, binary.LittleEndian, &block_size)
		log.Print("Reading block with size: ", block_size)

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Panic("Error: ", err)
		}

		// BUG! TODO
		if block_size > 10*1024*1024 {
			log.Print("There is something I don't understand about the format..")
			break
		}

		r := io.LimitReader(reader, block_size)
		round_1 := lz4.NewReader(r)
		round_2 := lz4.NewReader(round_1)
		decompressed_reader := round_2
		
		//decompressed_reader := r
		
		defer func() {
			if e := recover(); e != nil {
				offset, _ := fd.Seek(0, os.SEEK_CUR)
				buf, err := reader.Peek(8)
				log.Printf("Bytes: %q err: %q", buf, err)
				log.Print("!Reading from location ", offset, " - ", 	
						  offset-int64(reader.Buffered()), " buf = ", 
						  reader.Buffered(), " offs = ", offset)
				log.Panic("Panicked = ", e, " bytes remain: ", r.(*io.LimitedReader).N)
			}
		}()

		parse_block(decompressed_reader, &data)

		if *nrec != 0 && int64(len(data.access)) > *nrec {
			break
		}
	}

	log.Print("Read ", len(data.access))

	return data
}

func main() {

	if (*verbose) {
		log.Print("Startup")
		defer log.Print("Shutdown")
	}

	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("Wrong number of arguments, expected 1, got ", flag.NArg())
	}

	data := parse_file(flag.Arg(0))

	cleanup := make_window(400, 400, "Memory Accesses")
	defer cleanup()

	target_fps := 60

	main_loop(target_fps, data)
}
