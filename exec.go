// Code for handling executable file contents, determining symbols, etc

package main

import (
	"bytes"
	"debug/elf"
	"path/filepath"
	"log"
	"os"
	"strings"
)

type Binary struct {
	pathname string
	elf *elf.File
	symbolmap map[uint64] *elf.Symbol
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
				if exists(debug_path) { return debug_path }
				
				debug_path = "/usr/lib/debug" + file_dir + "/" + debugname
				if exists(debug_path) { return debug_path }
				
				debug_path = ("/usr/lib/debug" + 
							  strings.Replace(file_dir, "lib64", "lib", -1) + 
							  "/" + debugname)
				if exists(debug_path) { return debug_path }
			}
			break
		}
	}
	return ""
}

func NewBinary(path string) *Binary {
	file, err := elf.Open(path)
	if err != nil { log.Panic("Problem loading elf: ", err, " at ", path) }
		
	debug_filename := GetDebugFilename(path, file)
	if debug_filename != "" {
		//log.Panic("Debug filename: ", debug_filename)
		file, err = elf.Open(debug_filename)
		if err != nil { log.Panic("Problem loading elf: ", err, " at ", debug_filename) }
	}
	
	result := &Binary{path, file, make(map[uint64] *elf.Symbol)}
	
	virtoffset := uint64(0)
	
	for i := range file.Progs {
		prog := file.Progs[i]
		if prog.Type == elf.PT_LOAD {
			virtoffset = prog.Vaddr
			break
		}
	}
	
	var syms []elf.Symbol
	
	func () {
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
		result.symbolmap[s.Value - virtoffset] = s
	}
	
	return result
}

// Memoize binary data
var loaded_binaries map[string] *Binary
func init() {
	loaded_binaries = make(map[string] *Binary)
}

func (r *MemRegion) GetBinary() *Binary {
	if r.pathname == "unknown" { return nil }
	if binary, ok := loaded_binaries[r.pathname]; ok { return binary }
	binary := NewBinary(r.pathname)
	loaded_binaries[r.pathname] = binary
	return binary
}

func (r *MemRegion) GetSymbol(addr uint64) string {
	binary := r.GetBinary()
	if binary == nil { return "nil" }
	
	sym, ok := binary.symbolmap[addr - r.low]
	if !ok {
		return "unk"
	}
	return demangle(sym.Name)
}
