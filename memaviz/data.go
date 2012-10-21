// data.go: handling the data we are going to visualize

package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/pwaller/go-clz4"

	"github.com/go-gl/glh"
)

type ProgramData struct {
	filename       string
	region         []MemRegion
	blocks         []*Block
	detail_request chan *Block
}

func NewProgramData(filename string) *ProgramData {
	data := &ProgramData{
		filename:       filename,
		detail_request: make(chan *Block, 1000),
	}

	fd, err := os.Open(filename)
	//defer fd.Close()
	if err != nil {
		log.Panic("Fatal error: ", err)
	}

	// Used buffered for the header and page table
	reader := bufio.NewReaderSize(fd, 10*1024*1024)
	data.ParseHeader(reader)
	data.ParsePageTable(reader)
	_, err = fd.Seek(-int64(reader.Buffered()), 1)
	if err != nil {
		log.Panic(err)
	}

	// TODO: Record block start offsets so that they can be jumped back to

	go data.ParseBlocks(fd)

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

	last := MemRegion{}

	for i := range page_table_lines {
		line := page_table_lines[i]
		if len(line) == 0 {
			continue
		}
		x := MemRegion{data: data}

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
		if !(last.low < x.low) {
			panic("Expecting map regions to be sorted")
		}
		last = x
	}

}

var nblocks = int64(0)

func (data *ProgramData) ParseBlocks(reader io.ReadSeeker) {
	// These buffers must have a maximum capacity which can fit whatever we 
	// throw at them, and the rounds must have an initial length so that
	// the first byte can be addressed.

	input := make([]byte, 0, 10*1024*1024)
	round_1 := make([]byte, 1, 10*1024*1024)

	tell := func() int64 {
		// Get position in input
		where, err := reader.Seek(0, 1)
		if err != nil {
			log.Panic("Failed to tell(): %q", err)
		}
		return where
	}

	new_block := make(chan *Block)
	go func() {
		// Recieves new blocks, does heavy lifting for them, then appends
		// them to the list of blocks which the ProgramData is aware of.

		current_context := make(Records, 0)
		for b := range new_block {
			b.full_data = data
			b.context_records = current_context
			if *use_stree {
				b.stack_stree, current_context = b.BuildStree()
			}
			b.ActiveRegionIDs()
			b.vertex_data = b.GenerateVertices()
			b.RequestTexture()

			main_thread_work <- func(b *Block) func() {
				return func() {
					data.blocks = append(data.blocks, b)
				}
			}(b)
		}
	}()

	read_block := func() bool {
		// Read bytes from disk into `input` slice

		var block_size int64
		err := binary.Read(reader, binary.LittleEndian, &block_size)
		if err == io.EOF {
			return false
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
		return true
	}

	decode_records := func(block *Block, input []byte) {
		// Allocate block.records, decode `input` into that field

		exected_nrec := 10 * 1024 * 1024 / 56
		block.records = make(Records, exected_nrec)

		// TODO: use known output size decompression, allegedly faster..
		clz4.UncompressUnknownOutputSize(input, &round_1)
		clz4.UncompressUnknownOutputSize(round_1, block.records.AsBytes())

		block.nrecords = int64(len(block.records))
	}

	create_new_block := func(block_offset int64) {
		// A wild block appears!

		block := &Block{file_offset: block_offset}

		decode_records(block, input)

		new_block <- block
		nblocks++
	}

	drain_queue := func(current_offset int64) {
		for {
			select {
			case block := <-data.detail_request:
				reader.Seek(block.file_offset, 0)

				if !read_block() {
					log.Panic("Unexpected failure in read_block")
				}

				decode_records(block, input)
				block.vertex_data = block.GenerateVertices()
				block.requests.vertices = sync.Once{}

				// Continue where we left off
				reader.Seek(current_offset, 0)

			default:
				// No requests came in, move along!
				return
			}
		}
	}

	for {
		BlockUnlessSpareRAM(500)

		current_offset := tell()

		drain_queue(current_offset)

		if !read_block() {
			// EOF. Nothing more to be read
			break
		}
		create_new_block(current_offset)
	}

	for {
		drain_queue(0)
	}
}

func (data *ProgramData) GetRegion(addr uint64) *MemRegion {

	bi := sort.Search(len(data.region), func(i int) bool { return data.region[i].low > addr })

	// TODO: More efficient implementation
	for i := range data.region {
		r := &data.region[i]
		if r.low <= addr && addr < r.hi {
			if bi != i {
				panic("bi != i, binary search didn't agree with stupid search")
			}
			return r
		}
	}
	return &MemRegion{addr, addr, "-", "-", "-", "-", "unknown", data}
}

func (data *ProgramData) Draw(start_index, n int64) {
	nperblock := int64(10 * 1024 * 1024 / 56)
	start_block := start_index / nperblock
	if start_block < 0 {
		start_block = 0
	}
	n_blocks := n/nperblock + 2
	if n_blocks < 1 {
		n_blocks = 1
	}
	if start_block+n_blocks >= int64(len(data.blocks)) {
		n_blocks = int64(len(data.blocks)) - start_block
	}

	// Threshold above which we use the full block detail
	detailed := false
	if n_blocks < 20 {
		detailed = true
	}

	// TODO
	// * Defer drawing through a block summary
	// Idea: use a drawing order that goes center-out so that we don't notice
	//       blocks being loaded
	// 3
	// 1
	// 0
	// 2
	// 4

	glh.With(&Timer{Name: "DrawBlocks"}, func() {
		for i := range ints(start_block, n_blocks) {
			b := data.blocks[i]
			from, N := start_index-int64(i)*b.nrecords, b.nrecords
			b.Draw(from, N, detailed)
		}
	})
}

func (data *ProgramData) GetRecord(i int64) *Record {
	// TODO: determine block from `i`
	return nil
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
