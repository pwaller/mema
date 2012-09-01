package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
	"strings"
	"unsafe"
	
	"github.com/banthar/gl"
	"github.com/toberndo/go-stree/stree"
)

const (
	MEMA_ACCESS = 0
	MEMA_FUNC_ENTER = 1
	MEMA_FUNC_EXIT = 2
)

type Record struct {
	Type int64
	// Magic int64
	Content [48]byte // union { MemAccess, FunctionCall }
}

func (r *Record) MemAccess() *MemAccess {
	return (*MemAccess)(unsafe.Pointer(&r.Content[0]))
}

func (r *Record) FunctionCall() *FunctionCall {
	return (*FunctionCall)(unsafe.Pointer(&r.Content[0]))
}

var DummyRecord Record

func RecordSize() int {
	return int(unsafe.Sizeof(DummyRecord))
}

type Records []Record

func (records *Records) FromBytes(bslice []byte) {
	if len(bslice) % RecordSize() != 0 {
		log.Panic("Unexpectedly have some bytes left over.. n=", 
				  len(bslice) % RecordSize())
	}
	n_records := len(bslice) / RecordSize()
	records_header := (*reflect.SliceHeader)(unsafe.Pointer(records))
	records_header.Data = uintptr(unsafe.Pointer(&bslice[0]))
	records_header.Len = n_records
	records_header.Cap = n_records
}

func (r Record) String() string {
	if r.Type == MEMA_ACCESS {
		a := r.MemAccess()
		//return fmt.Sprintf("r=%d/%x MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
			//r.Type, r.Magic, a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
		return fmt.Sprintf("r=%d MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
			r.Type, a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
	}
	f := r.FunctionCall()
	//return fmt.Sprintf("r=%d/%x FunctionCall{ptr=0x%x}",
		//r.Type, r.Magic, f.FuncPointer)
	return fmt.Sprintf("r=%d FunctionCall{ptr=0x%x}",
		r.Type, f.FuncPointer)
}

type FunctionCall struct {
	FuncPointer uint64
}

type MemAccess struct {
	Time			 float64
	Pc, Bp, Sp, Addr uint64
	IsWrite			 uint64 // because alignment.
}

func (a MemAccess) String() string {
	return fmt.Sprintf("MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
		a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
}

type MemRegion struct {
	low, hi uint64
	perms, offset, dev, inode, pathname string
}

func (r *MemRegion) String() string {
	return fmt.Sprintf("%x-%x %s", r.low, r.hi, r.pathname)
}

type ProgramData struct {
	filename string
	region []MemRegion
	access []MemAccess
	records Records
	nrecords int64
	quiet_pages map[uint64] bool
	active_pages map[uint64] bool
	n_pages_to_left map[uint64] uint64
	n_inactive_to_left map[uint64] uint64
	vertex_data []*ColorVertices
	stack_stree *stree.Tree
}

func (data *ProgramData) GetRegion(addr uint64) *MemRegion {
	for i := range data.region {
		r := &data.region[i]
		if r.low <= addr && addr < r.hi { return r }
	}
	return &MemRegion{addr, addr, "-", "-", "-", "-", "unknown"}
}

func NewProgramData(filename string) *ProgramData {
	data := &ProgramData{}
	
	data.filename = filename
	
	fd, err := os.Open(filename)
	defer fd.Close()
	if err != nil {
		log.Panic("Fatal error: ", err)
	}
	
	reader := bufio.NewReaderSize(fd, 10*1024*1024)

	data.ParseHeader(reader)	
	data.ParsePageTable(reader)
	
	// TODO: Stripe across the file to find where blocks are
	// TODO: Load on demand sections which will be read
	data.ParseBlocks(reader)
	
	// TODO: building the stree isn't going to work well with striping across file
	log.Print("Loading stree..")
	data.stack_stree = data.BuildStree()
	log.Print("Loaded stree")

	stack := (*data.stack_stree).Query(100, 100)
	log.Print(" -- stree test:", stack)

	log.Print("Read ", len(data.access), " records")
	
	active_regions := data.ActiveRegionIDs()
	if *verbose {
		log.Printf("Have %d active regions", len(active_regions))
	}
	
	if *debug {
		log.Print("Region info:")
		for i := range data.region {
			log.Print(" ", data.region[i])
		}
	}

	return data
}

func (data *ProgramData) ParseHeader(reader *bufio.Reader) {
	magic_buf := make([]byte, 8)
	_, err := reader.Read(magic_buf)
	if err != nil || string(magic_buf) != "MEMACCES" {
		log.Panic("Error reading magic bytes: ", err, " bytes=", magic_buf)
	}
}

func (data *ProgramData) ParsePageTable(reader *bufio.Reader) {
	page_table_bytes, err := reader.ReadBytes('\x00')
	if err != nil {
		log.Panic("Error reading page table: ", err)
	}
	if *verbose {
		log.Printf("Page table size: %d bytes", len(page_table_bytes))
	}

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
}

func (data *ProgramData) ParseBlocks(reader *bufio.Reader) {
	// These buffers must have a maximum capacity which can fit whatever we 
	// throw at them, and the rounds must have an initial length so that
	// the first byte can be addressed.
	input := make([]byte, 0, 20*1024*1024)
	round_1 := make([]byte, 1, 20*1024*1024)
	round_2 := make([]byte, 1, 20*1024*1024)

	for {
		var block_size int64
		err := binary.Read(reader, binary.LittleEndian, &block_size)
		if err == io.EOF { break }
		if err != nil { log.Panic("Error: ", err) }
		
		if *debug { log.Print("Reading block with size: ", block_size) }
		
		r := io.LimitReader(reader, block_size)
		input = input[0:block_size]
		n, err := io.ReadFull(r, input)
		
		if int64(n) != block_size {
			log.Panicf("Err = %q, expected %d, got %d", err, block_size, n)
		}
		LZ4_uncompress_unknownOutputSize(input, &round_1)
		LZ4_uncompress_unknownOutputSize(round_1, &round_2)

		var records Records
		records.FromBytes(round_2)
		data.records = append(data.records, records...)
		data.nrecords += int64(len(records))

		// Read only a limited number of records > nrec, if set.
		if *nrec != 0 && int64(data.nrecords) > *nrec { break }
	}
	log.Print("Total records: ", data.nrecords)
}

func (data *ProgramData) ParseBlock(bslice []byte) {
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

func (d *ProgramData) RegionID(addr uint64) int {
	for i := range d.region {
		if d.region[i].low < addr && addr < d.region[i].hi { return i }
	}
	//log.Panicf("Address 0x%x not in any defined memory region!", addr)
	return len(d.region)
}

func (d *ProgramData) ActiveRegionIDs() []int {
	// TODO: Quite a bite of this wants to be moved into NewProgramData
	active := make(map[int] bool)
	page_activity := make(map[uint64] uint)
	
	for i := range d.records {
		r := &d.records[i]
		if r.Type != MEMA_ACCESS { continue }
		a := r.MemAccess()
		//a := &d.access[i]
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
	var highest_page_activity uint = 0
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
		if *hide_qp_fraction != 0 &&
		   value < highest_page_activity / *hide_qp_fraction {
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
	}
	
	return result
}

func (data *ProgramData) Draw(start, N int64) bool {
	defer OpenGLSentinel()()
	
	width := uint64(len(data.active_pages)) * *PAGE_SIZE
	if width == 0 { width = 1 }
	
	gl.LineWidth(1)
	
	if *pageboundaries {
		
		var vc ColorVertices
		boundary_color := Color{64, 64, 64, 255}
		
		if width / *PAGE_SIZE < 1000 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p < width; p += *PAGE_SIZE {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				vc.Add(ColorVertex{boundary_color, Vertex{x, -2}})
				vc.Add(ColorVertex{boundary_color, Vertex{x,  2}})
			}
		}
		
		vc.Draw()
	}
	
	gl.LineWidth(1)

	if cap(data.vertex_data) == 0 {
		log.Print("start = ", start)
		data.vertex_data = make([]*ColorVertices, 1)
		data.vertex_data[0] = data.GetAccessVertexData(0, int64(data.nrecords))
	}
	
	gl.PushMatrix()
	
	gl.Translated(0, -2, 0)
	gl.Scaled(1, 4 / float64(*nback), 1)
	gl.Translated(0, -float64(start), 0)
	OpenGLSentinel()
	
	data.vertex_data[0].DrawPartial(start, *nback)
	gl.PopMatrix()
	OpenGLSentinel()
	
	NV := int64(len(*data.vertex_data[0]))
	
	var eolmarker ColorVertices
	
	// End-of-data marker
	if start + N > NV {
		y := -2 + 4*float32(NV - start) / float32(*nback)
		c := Color{255, 255, 255, 255}
		eolmarker.Add(ColorVertex{c, Vertex{-2, y}})
		eolmarker.Add(ColorVertex{c, Vertex{ 2, y}})
	}
	
	OpenGLSentinel()
	
	gl.LineWidth(1)
	eolmarker.Draw()
	
	return start > NV //int64(data.nrecords)
}

func (data *ProgramData) GetAccessVertexData(start, N int64) *ColorVertices {

	width := uint64(len(data.active_pages)) * *PAGE_SIZE
	
	vc := &ColorVertices{}
		
	// TODO: Do coordinate scaling on the GPU (modify orthographic scaling)
	// TODO: Transport vertices to the GPU in bulk using glBufferData
	//	   Function calls here appear to be the biggest bottleneck
	var stack_depth int
	
	for pos := start; pos < min(start + N, int64(data.nrecords)); pos++ {
		if pos < 0 { continue }
		
		r := &data.records[pos]
		if r.Type == MEMA_ACCESS {	
			// take it
		} else if r.Type == MEMA_FUNC_ENTER {
			stack_depth++
			
			y := float32(int64(len(*vc)) - start)
			c := Color{64, 64, 255, 255}
			vc.Add(ColorVertex{c, Vertex{2.1 + float32(stack_depth) / 80., y}})
			
			continue
		} else if r.Type == MEMA_FUNC_EXIT {
			
			y := float32(int64(len(*vc)) - start)
			c := Color{255, 64, 64, 255}
			vc.Add(ColorVertex{c, Vertex{2.1 + float32(stack_depth) / 80., y}})
			
			stack_depth--
			
			continue
		} else {
			log.Panic()
		}
		a := r.MemAccess()
		
		page := a.Addr / *PAGE_SIZE
		if _, present := data.quiet_pages[page]; present {
			continue
		}
		
		x := float32(a.Addr - data.n_inactive_to_left[page] * *PAGE_SIZE) / float32(width)
		x = (x - 0.5) * 4
		
		if x > 4 || x < -4 {
			log.Panic("x has unexpected value: ", x)
		}
		
		y := float32(int64(len(*vc)) - start)
		
		c := Color{uint8(a.IsWrite)*255, uint8(1-a.IsWrite)*255, 0, 255}
		
		vc.Add(ColorVertex{c, Vertex{x, y}})
		
		/*
		TODO: Reintroduce 'recently hit memory locations'
		if pos > (start + N) - N / 20 {
			vc.Add(ColorVertex{c, Vertex{x, 2 + 0.1}})
			vc.Add(ColorVertex{c, Vertex{x, 2}})
		}
		*/
	}
	
	return vc
}
