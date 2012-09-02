// +build cgo

package main

import (
	//"log"
	"unsafe"
)

// #cgo LDFLAGS: -lstdc++
// // #include <stddef.h>
// #include <stdlib.h>
// char* __cxa_demangle(const char* __mangled_name, char* __output_buffer,
//					    size_t* __length, int* __status);
import "C"

func demangle(sym string) string {

	csym := C.CString(sym)
	defer C.free(unsafe.Pointer(csym))
	
	var status C.int
	
	// __cxa_demangle has a length field, but it's the length of the buffer,
	// not the length of the resulting string.
	cdemangled := C.__cxa_demangle(csym, nil, nil, &status)
	if status != C.int(0) {
		// Demangling failed for some reason (e.g, it's not a mangled sym)
		return sym
	}
	defer C.free(unsafe.Pointer(cdemangled))
		
	sym = C.GoString(cdemangled)
	
	return sym
}
