package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"unsafe"
	
	"github.com/banthar/gl"
)

const (
	MEMA_ACCESS = 0
	MEMA_FUNC_ENTER = 1
	MEMA_FUNC_EXIT = 2
)

type MemRegion struct {
	low, hi uint64
	perms, offset, dev, inode, pathname string
}

type Record struct {
	Type, Magic int64
	Content [48]byte // union { MemAccess, FunctionCall }
}

var DummyRecord Record

func RecordSize() int {
	return int(unsafe.Sizeof(DummyRecord))
}

type FunctionCall struct {
	FuncPointer uint64
}

type MemAccess struct {
	Time			 float64
	Pc, Bp, Sp, Addr uint64
	IsWrite			 uint64 // because alignment.
}

func (r *Record) MemAccess() *MemAccess { return (*MemAccess)(unsafe.Pointer(&r.Content[0])) }
func (r *Record) FunctionCall() *FunctionCall { return (*FunctionCall)(unsafe.Pointer(&r.Content[0])) }

type ProgramData struct {
	region []MemRegion
	access []MemAccess
	quiet_pages map[uint64] bool
	active_pages map[uint64] bool
	n_pages_to_left map[uint64] uint64
	n_inactive_to_left map[uint64] uint64
}

func (a MemAccess) String() string {
	return fmt.Sprintf("MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
		a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
}

func (r Record) String() string {
	if r.Type == MEMA_ACCESS {
		a := r.MemAccess()
		return fmt.Sprintf("r=%d/%x MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
			r.Type, r.Magic, a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
	}
	f := r.FunctionCall()
	return fmt.Sprintf("r=%d/%x FunctionCall{ptr=0x%x}",
		r.Type, r.Magic, f.FuncPointer)
}

func (d ProgramData) RegionID(addr uint64) int {
	for i := range d.region {
		if d.region[i].low < addr && addr < d.region[i].hi { return i }
	}
	//log.Panicf("Address 0x%x not in any defined memory region!", addr)
	return len(d.region)
}


func (d *ProgramData) ActiveRegionIDs() []int {
	active := make(map[int] bool)
	page_activity := make(map[uint64] int)
	
	for i := range d.access {
		a := &d.access[i]
		active[d.RegionID(a.Addr)] = true
		page := a.Addr / *PAGE_SIZE
		
		page_activity[page]++
	}
	result := make([]int, 0)
	for k, _ := range active {
		result = append(result, k)
	}
	sort.Ints(result)
	
	// Figure out how much activity the busiest page has
	highest_page_activity := 0
	for _, value := range page_activity {
		if value > highest_page_activity {
			highest_page_activity = value
		}
	}

	d.quiet_pages = make(map[uint64] bool)
	d.active_pages = make(map[uint64] bool)
	d.n_pages_to_left = make(map[uint64] uint64)
	d.n_inactive_to_left = make(map[uint64] uint64)
	
	// Populate the "quiet pages" map (less than 1% of the activity of the 
	// most active page)
	for page, value := range page_activity {
		if value < highest_page_activity / 100 {
			d.quiet_pages[page] = true
			continue
		}
		d.active_pages[page] = true
	}
	
	log.Print("Quiet pages: ", len(d.quiet_pages), " active: ", len(d.active_pages))
	
	// Build list of pages which are active, count active pages to the left
	active_pages := make([]uint64, len(d.active_pages))
	i := 0
	for page, _ := range d.active_pages {
		active_pages[i] = page
		i++
	}
	sort.Sort(UInt64Slice(active_pages))
	
	for i := range active_pages {
		page := active_pages[i]
		d.n_pages_to_left[page] = uint64(i)
	}

	log.Print("Active pages: ", len(active_pages))
	var total_inactive_to_left uint64 = 0
	for i = range active_pages {
		p := active_pages[i]
		var inactive_to_left uint64
		if i == 0 {
			inactive_to_left = active_pages[i]
		} else {
			inactive_to_left = active_pages[i] - active_pages[i-1]
		}
		total_inactive_to_left += inactive_to_left - 1
		d.n_inactive_to_left[p] = total_inactive_to_left
		log.Print("Inactive to left ", p, " = ", inactive_to_left)
	}
	
	return result
}


func (data ProgramData) Draw(start, N int64, region_id int) bool {

	//log.Print("Enter Draw")
	//region := data.region[region_id]

	var minAddr, maxAddr uint64 = math.MaxUint64, 0

	for pos := start; pos < min(start + N, int64(len(data.access))); pos++ {
		if pos < 0 { continue }
		r := &data.access[pos]
		if _, present := data.quiet_pages[r.Addr / *PAGE_SIZE]; present {
			//log.Print("Ignoring addr: ", r.Addr)
			continue
		}
		if data.RegionID(r.Addr) != region_id { continue }
		if r.Addr < minAddr {
			minAddr = *PAGE_SIZE * (r.Addr / *PAGE_SIZE)
		}
		if r.Addr > maxAddr {
			maxAddr = *PAGE_SIZE * (r.Addr / *PAGE_SIZE + 1)
		}
	}
	
	// Use first active page boundary as minaddr
	minAddr = data.n_inactive_to_left[minAddr / *PAGE_SIZE] * *PAGE_SIZE
	
	//log.Print("Finished pruning..")

	//width := maxAddr - minAddr
	width := uint64(len(data.active_pages)) * *PAGE_SIZE
	if width == 0 { width = 1 }

	if *pageboundaries {
		if width / *PAGE_SIZE < 100 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p < width; p += *PAGE_SIZE {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				gl.Color4d(0.25, 0.25, 0.25, 1)
				x *= margin_factor
				gl.Vertex3f(x, -2, -10)
				gl.Vertex3f(x, 2, -10)
			}
		}
	}
	//return false
	//log.Print("Enter loop")

	for pos := start; pos < min(start + N, int64(len(data.access))); pos++ {
		if pos < 0 { continue }
		r := &data.access[pos]
		//i := data.RegionID(r.Addr)
		page := r.Addr / *PAGE_SIZE
		//if i != region_id { continue }
		if _, present := data.quiet_pages[r.Addr / *PAGE_SIZE]; present {
			//log.Print("Ignoring addr: ", r.Addr)
			//continue
		}
		//log.Print("  Drawing line..")
		x := float32(r.Addr - data.n_inactive_to_left[page] * *PAGE_SIZE) / float32(width)
		x = (x - 0.5) * 4
		//log.Print("Position: ", x)
		x *= margin_factor
		y := -2 + 4*float32(pos - start) / float32(N)
		gl.Color4d(float64(r.IsWrite), float64(1-r.IsWrite), 0, 1+math.Log(1.-float64(N - (pos - start))/float64(N))/3)
		//log.Print("iswrite: ", r.IsWrite, " ", float64(1-r.IsWrite), float64(r.IsWrite))
		gl.Vertex3f(x, y, -10)
		//gl.Vertex3f(x+(float32(r.IsWrite)-0.5)*0.05, y+0.0125, -10)
		gl.Vertex3f(x, y+0.0125, -10)
		gl.Color4d(float64(r.IsWrite), float64(1-r.IsWrite), 0, 1)
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


type Records []Record
func (records *Records) FromBytes(bslice []byte) {
	if len(bslice) % RecordSize() != 0 {
		log.Panic("Unexpectedly have some bytes left over.. n=", len(bslice) % RecordSize())
	}
	n_records := len(bslice) / RecordSize()
	records_header := (*reflect.SliceHeader)(unsafe.Pointer(records))
	records_header.Data = uintptr(unsafe.Pointer(&bslice[0]))
	records_header.Len = n_records
	records_header.Cap = n_records
}

func parse_block(bslice []byte, data *ProgramData) {
	var records Records
	records.FromBytes(bslice)

	for i := range records {
		r := &records[i]
		// TODO: Something with the other record types
		if r.Type == MEMA_ACCESS {
			data.access = append(data.access, *r.MemAccess())
		}
		i++
		continue
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

	input := make([]byte, 0, 20*1024*1024)
	round_1 := make([]byte, 1, 20*1024*1024)
	round_2 := make([]byte, 1, 20*1024*1024)

	for {

		//offset, err := fd.Seek(0, os.SEEK_CUR)
		//log.Print("Reading from location ", offset-int64(reader.Buffered()), " buf = ", reader.Buffered(), " offs = ", offset)
		//buf, err := reader.Peek(8)
		//log.Printf("Bytes: %q", buf)

		var block_size int64
		err = binary.Read(reader, binary.LittleEndian, &block_size)
		log.Print("Reading block with size: ", block_size)

		if err == io.EOF { break }
		if err != nil { log.Panic("Error: ", err) }

		r := io.LimitReader(reader, block_size)
		input = input[0:block_size]
		
		if *compressed {
			n, err := io.ReadFull(r, input)
			if int64(n) != block_size {
				log.Panicf("Err = %q, expected %d, got %d", err, block_size, n)
			}
			LZ4_uncompress_unknownOutputSize(input, &round_1)
			LZ4_uncompress_unknownOutputSize(round_1, &round_2)
		}

		parse_block(round_2, &data)

		if *nrec != 0 && int64(len(data.access)) > *nrec {
			break
		}
	}

	log.Print("Read ", len(data.access))

	return data
}
