package main

import (
	"image"
	"image/png"
	"log"
	"os"
	"reflect"
	"runtime"
	"sort"
	"time"

	"github.com/go-gl/gl"
)

import (
	//"bufio"
	"fmt"
	"io"
	//"net/textproto"
)

type UInt64Slice []uint64

func (p UInt64Slice) Len() int           { return len(p) }
func (p UInt64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p UInt64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p UInt64Slice) Sort() { sort.Sort(p) }

func min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}

func Capture() {
	// TODO: co-ordinates, filename, cleverness to stitch many together
	im := image.NewNRGBA(image.Rect(0, 0, 400, 400))
	gl.ReadBuffer(gl.BACK_LEFT)
	gl.ReadPixels(0, 0, 400, 400, gl.RGBA, gl.UNSIGNED_BYTE, im.Pix)

	fd, err := os.Create("test.png")
	if err != nil {
		log.Panic("Err: ", err)
	}
	defer fd.Close()

	png.Encode(fd, im)
}

func SortedMapKeys(input interface{}) []string {
	keys := reflect.ValueOf(input).MapKeys()
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = k.String()
	}
	sort.Strings(result)
	return result
}

func init() {
	n := runtime.NumCPU() + 1
	runtime.GOMAXPROCS(n)
	log.Printf("GOMAXPROCS set to %d", n)
}

func meminfo() (total, free, buffers, cached int64) {
	fd, err := os.Open("/proc/meminfo")
	defer fd.Close()
	if err != nil {
		panic(err)
	}

	//lreader := textproto.NewReader(bufio.NewReader(fd))

	mapping := map[string]*int64{
		"MemTotal": &total,
		"MemFree":  &free,
		"Buffers":  &buffers,
		"Cached":   &cached,
	}

	var name, unit string
	var value int64

	for {

		n, err := fmt.Fscanln(fd, &name, &value, &unit)
		//log.Print("Read line: ", name, " v: ", value, unit)
		if err == io.EOF {
			break
		}
		if n < 2 {
			continue
		}
		what, present := mapping[name[:len(name)-1]]
		if present {
			*what = value * 1024
		}
	}

	return
}

var memstats *runtime.MemStats = new(runtime.MemStats)

func SystemFree() int64 {
	total, free, buffers, cached := meminfo()
	_, _ = total, buffers
	return free + cached
}

// Returns the number of spare megabytes of ram after leaving 100 + 10% spare
func SpareRAM() int64 {
	const GRACE_ABS = 100 // MB
	const GRACE_REL = 20  // %

	runtime.ReadMemStats(memstats)

	total, free, _, cached := meminfo()

	/*
		si := &syscall.Sysinfo_t{}
		err := syscall.Sysinfo(si)
		if err != nil {
			return -999913379999
		}
	*/
	grace := int64(GRACE_REL*total/100 + GRACE_ABS)
	/*
		free := int64(si.Freeram + si.Bufferram)
		log.Print("Free: ", free/1024/1024, " grace: ", grace/1024/1024, " freeram: ", si.Freeram/1024/1024, "bufram: ", si.Bufferram/1024/1024)
		log.Printf("%+v", si)
	*/

	allocated_but_unused := int64(memstats.HeapIdle)

	//log.Print("Grace: ", grace, " free: ", free)
	return (free + cached - grace + allocated_but_unused) / 1024 / 1024
}

func BlockUnlessSpareRAM(needed_mb int64) {
	for {
		spare := SpareRAM()
		if spare >= needed_mb {
			break
		}
		time.Sleep(100 * time.Microsecond)
	}
}

func ints(low, n int64) <-chan int64 {
	result := make(chan int64)
	go func() {
		for i := low; i < low+n; i++ {
			result <- i
		}
		close(result)
	}()
	return result
}

func GCStats() {
	memstats := new(runtime.MemStats)
	runtime.ReadMemStats(memstats)
	log.Printf("  -- paused for %v -- total %v -- N %d",
		time.Duration(memstats.PauseNs[(memstats.NumGC-1)%256]),
		time.Duration(memstats.PauseTotalNs), memstats.NumGC)
}
