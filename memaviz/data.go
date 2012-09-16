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
	"syscall"
	"time"

	"github.com/pwaller/go-clz4"
)

type ProgramData struct {
	filename string
	region   []MemRegion
	blocks   []*Block

	//update              chan<- bool
	//new_block_available <-chan *Block
	//request_new_block   chan<- int
}

func NewProgramData(filename string) *ProgramData {
	data := &ProgramData{}

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
	//defer fd.Close()
	if err != nil {
		log.Panic("Fatal error: ", err)
	}

	reader := bufio.NewReaderSize(fd, 10*1024*1024)

	data.ParseHeader(reader)
	data.ParsePageTable(reader)

	// TODO: Record block start offsets so that they can be jumped back to

	new_block := make(chan *Block)
	go data.ParseBlocks(reader, new_block)
	go func() {
		current_context := make(Records, 0)
		for b := range new_block {
			b.full_data = data
			b.context_records = current_context
			b.stack_stree, current_context = b.BuildStree()
			b.ActiveRegionIDs()

			main_thread_work <- func(b *Block) func() {
				return func() {
					data.blocks = append(data.blocks, b)
				}
			}(b)
		}
	}()

	if *debug {
		log.Print("Region info:")
		for i := range data.region {
			log.Print(" ", data.region[i])
		}
	}

	return data
}

func (data *ProgramData) ParseHeader(reader io.Reader) {
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

// Returns the number of spare megabytes of ram after leaving 100 + 10% spare
func SpareRAM() int64 {
	const GRACE_ABS = 100 // MB
	const GRACE_REL = 10  // %

	si := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(si)
	if err != nil {
		return -999913379999
	}
	grace := int64(GRACE_REL*si.Totalram/100 + GRACE_ABS)
	free := int64(si.Freeram + si.Bufferram)
	//log.Print("Grace: ", grace, " free: ", free)
	return (free - grace) / 1024 / 1024
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

func (data *ProgramData) ParseBlocks(reader io.Reader, new_block chan<- *Block) {
	// These buffers must have a maximum capacity which can fit whatever we 
	// throw at them, and the rounds must have an initial length so that
	// the first byte can be addressed.

	blocks := int64(0)
	input := make([]byte, 0, 10*1024*1024)
	round_1 := make([]byte, 1, 10*1024*1024)

	for {
		BlockUnlessSpareRAM(10)

		blocks++
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

		input = input[0:block_size]
		n, err := io.ReadFull(reader, input)

		if int64(n) != block_size {
			log.Panicf("Err = %q, expected %d, got %d", err, block_size, n)
		}
		// TODO: use known output size decompression
		clz4.LZ4_uncompress_unknownOutputSize(input, &round_1)

		block := &Block{}

		block.records = make(Records, 10*1024*1024/56)
		clz4.LZ4_uncompress_unknownOutputSize(round_1, block.records.AsBytes())
		block.nrecords += int64(len(block.records))

		new_block <- block
	}
}

func (data *ProgramData) GetRegion(addr uint64) *MemRegion {
	// TODO: More efficient implementation
	for i := range data.region {
		r := &data.region[i]
		if r.low <= addr && addr < r.hi {
			return r
		}
	}
	return &MemRegion{addr, addr, "-", "-", "-", "-", "unknown"}
}

func (data *ProgramData) Draw(start_index, n int64) {
	// TODO: determine blocks which are visible on screen

	With(&Timer{Name: "DrawBlocks"}, func() {
		for i, b := range data.blocks {
			b.Draw(start_index-int64(i)*b.nrecords, b.nrecords)
			if false && i > 50 {
				break
			}
		}
	})
}

func (data *ProgramData) GetRecord(i int64) *Record {
	// TODO: determine block from `i`
	if i >= 0 && i < data.blocks[0].nrecords {
		return &data.blocks[0].records[i]
	}
	return nil
}

func (data *ProgramData) GetStackNames(i int64) []string {
	records_per_block := (int64)(10 * 1024 * 1024 / 56)
	block_index := i / records_per_block
	internal_index := i % records_per_block
	if block_index >= 0 && block_index < int64(len(data.blocks)) {
		return data.blocks[block_index].GetStackNames(internal_index)
	}
	return []string{}
}
