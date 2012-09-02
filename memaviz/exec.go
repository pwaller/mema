// Code for handling executable file contents, determining symbols, etc

package main

import (
	"bytes"
	"debug/dwarf"
	"debug/elf"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/toberndo/go-stree/stree"
)

type Binary struct {
	pathname  string
	elf       *elf.File
	dwarf     *dwarf.Data
	symbolmap map[uint64]*elf.Symbol
	// TODO: Extend this to care for multiple entries with the same "Lowpc" value
	dwarf_entries map[uint64]*dwarf.Entry
	dwarf_stree   *stree.Tree
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GetDebugFilename(path string, file *elf.File) string {
	for i := range file.Sections {
		s := file.Sections[i]
		if s.Name == ".gnu_debuglink" {
			data, err := s.Data()
			if err == nil {
				nul := bytes.IndexByte(data, 0)
				debugname := string(data[:nul])

				// From http://sourceware.org/gdb/onlinedocs/gdb/Separate-Debug-Files.html
				// [not checked, don't know how to get BID.]
				// /usr/lib/debug/.build-id/ab/cdef1234.debug 
				// /usr/bin/ls.debug
				// /usr/bin/.debug/ls.debug
				// /usr/lib/debug/usr/bin/ls.debug

				file_dir := filepath.Dir(path)
				debug_path := file_dir + "/.debug/" + debugname
				if exists(debug_path) {
					return debug_path
				}

				debug_path = "/usr/lib/debug" + file_dir + "/" + debugname
				if exists(debug_path) {
					return debug_path
				}

				debug_path = ("/usr/lib/debug" +
					strings.Replace(file_dir, "lib64", "lib", -1) +
					"/" + debugname)
				if exists(debug_path) {
					return debug_path
				}
			}
			break
		}
	}
	return ""
}

func NewBinary(path string) *Binary {
	file, err := elf.Open(path)
	if err != nil {
		log.Panic("Problem loading elf: ", err, " at ", path)
	}

	debug_filename := GetDebugFilename(path, file)
	if debug_filename != "" {
		//log.Panic("Debug filename: ", debug_filename)
		file, err = elf.Open(debug_filename)
		if err != nil {
			log.Panic("Problem loading elf: ", err, " at ", debug_filename)
		}
	}

	dw, err := file.DWARF()
	if err != nil {
		log.Printf("!! No DWARF for %q err = %v", path, err)
		dw = nil
	}

	tree := stree.NewTree()
	result := &Binary{path, file, dw, make(map[uint64]*elf.Symbol),
		make(map[uint64]*dwarf.Entry), &tree}

	if dw != nil {
		//tree := result.dwarf_stree
		dwarf_entries := &result.dwarf_entries

		dr := dw.Reader()
		//log.Panic("Abort ", path, dwarf)
		i := 0
		n := 0
		for {
			i++
			//if i > 1000 { break }
			entry, err := dr.Next()
			if err != nil {
				log.Panic("Error reading dwarf: ", entry)
			}
			//log.Print("Got dwarf entry: ", entry)
			if entry == nil {
				break
			}

			lpc := entry.Val(dwarf.AttrLowpc)
			hpc := entry.Val(dwarf.AttrHighpc)
			if hpc != nil && lpc != nil {
				var hpcv, lpcv uint64
				hpcv = hpc.(uint64)
				lpcv = lpc.(uint64)

				tree.Push(int(lpcv), int(hpcv))
				(*dwarf_entries)[lpcv] = entry

				//log.Print("Got one: ", lpcv, hpcv, entry)
				n++
			}
		}
		log.Print("Building dwarf tree..")
		tree.BuildTree()
		log.Print("dwarf tree built.")

		//log.Panic("Abort, got ", n)
	}

	virtoffset := uint64(0)

	for i := range file.Progs {
		prog := file.Progs[i]
		if prog.Type == elf.PT_LOAD {
			virtoffset = prog.Vaddr
			break
		}
	}

	var syms []elf.Symbol

	func() {
		// populate symbolmap
		//var err error
		syms, err = file.Symbols()
		if err != nil {
			debugname := GetDebugFilename(path, file)
			log.Print("Got debug name: ", debugname)
		}
	}()

	for i := range syms {
		s := &syms[i]
		result.symbolmap[s.Value-virtoffset] = s
	}

	return result
}

// Memoize binary data
var loaded_binaries map[string]*Binary

func init() {
	loaded_binaries = make(map[string]*Binary)
}

func (r *MemRegion) GetBinary() *Binary {
	if r.pathname == "unknown" {
		return nil
	}
	if binary, ok := loaded_binaries[r.pathname]; ok {
		return binary
	}
	binary := NewBinary(r.pathname)
	loaded_binaries[r.pathname] = binary
	return binary
}

func (data *ProgramData) GetSymbol(addr uint64) string {
	region := data.GetRegion(addr)
	return region.GetSymbol(addr)
}

func (r *MemRegion) GetSymbol(addr uint64) string {
	binary := r.GetBinary()
	if binary == nil {
		return "nil"
	}

	sym, ok := binary.symbolmap[addr-r.low]
	if !ok {
		return "unk"
	}
	return demangle(sym.Name)
}

func (d *ProgramData) GetDwarf(addr uint64) []*dwarf.Entry {
	r := d.GetRegion(addr)
	return r.GetDwarf(addr)
}

func (r *MemRegion) GetDwarf(addr uint64) []*dwarf.Entry {
	binary := r.GetBinary()
	if binary == nil || binary.dwarf == nil {
		return make([]*dwarf.Entry, 0)
	}

	addr = addr - r.low

	intervals := (*binary.dwarf_stree).Query(int(addr), int(addr))
	log.Print("Query: n=", len(intervals), addr, int(addr))

	lower_bounds := make([]int, len(intervals))
	for i := range intervals {
		lower_bounds[i] = intervals[i].Segment.From
	}
	sort.Ints(lower_bounds)

	dwarf_entries := make([]*dwarf.Entry, len(intervals))

	for i := range lower_bounds {
		e := binary.dwarf_entries[uint64(lower_bounds[i])]
		dwarf_entries[i] = e
	}

	return dwarf_entries
}
