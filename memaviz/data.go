// data.go: handling the data we are going to visualize

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/pwaller/go-clz4"
)

type ProgramData struct {
	filename string
	region   []MemRegion
	blocks   []*Block

	b Block
	//update              chan<- bool
	//new_block_available <-chan *Block
	//request_new_block   chan<- int
}

func NewProgramData(filename string) *ProgramData {
	data := &ProgramData{}

	data.b.full_data = data
	/*
		data.update := make(chan bool)

		new_block_available := make(chan *Block)
		data.new_block_available = new_block_available

		go func() {
			var new_blocks []*Block
			for {
				select {
					case block := <- data.new_block_available:
						new_blocks = append(new_blocks, block)
					case <-data.update:
						data.blocks = append(data.blocks, new_blocks...)
						new_blocks = []*Block{}
				}
			}
		}()
	*/

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

	//data.BlockParser(reader)

	// TODO: building the stree isn't going to work well with striping across file
	log.Print("Loading stree.. ", len(data.b.records))
	data.b.stack_stree = data.b.BuildStree()
	log.Print("Loaded stree")

	stack := (*data.b.stack_stree).Query(100, 100)
	log.Print(" -- stree test:", stack)

	active_regions := data.b.ActiveRegionIDs()
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

func (data *ProgramData) GetRegion(addr uint64) *MemRegion {
	for i := range data.region {
		r := &data.region[i]
		if r.low <= addr && addr < r.hi {
			return r
		}
	}
	return &MemRegion{addr, addr, "-", "-", "-", "-", "unknown"}
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

	block := &data.b

	for {
		var block_size int64
		err := binary.Read(reader, binary.LittleEndian, &block_size)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Panic("Error: ", err)
		}

		if *debug {
			log.Print("Reading block with size: ", block_size)
		}

		r := io.LimitReader(reader, block_size)
		input = input[0:block_size]
		n, err := io.ReadFull(r, input)

		if int64(n) != block_size {
			log.Panicf("Err = %q, expected %d, got %d", err, block_size, n)
		}
		clz4.LZ4_uncompress_unknownOutputSize(input, &round_1)
		clz4.LZ4_uncompress_unknownOutputSize(round_1, &round_2)

		var records Records
		records.FromBytes(round_2)
		block.records = append(block.records, records...)
		block.nrecords += int64(len(records))

		// Read only a limited number of records > nrec, if set.
		if *nrec != 0 && int64(block.nrecords) > *nrec {
			break
		}
	}
	log.Print("Total records: ", block.nrecords)
}

func (d *ProgramData) RegionID(addr uint64) int {
	for i := range d.region {
		if d.region[i].low < addr && addr < d.region[i].hi {
			return i
		}
	}
	//log.Panicf("Address 0x%x not in any defined memory region!", addr)
	return len(d.region)
}
