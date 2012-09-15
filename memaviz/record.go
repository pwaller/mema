// record.go: handling individual mema instrumentation events

package main

import (
	"fmt"
	"log"
	"reflect"
	"unsafe"
)

const (
	MEMA_ACCESS     = 0
	MEMA_FUNC_ENTER = 1
	MEMA_FUNC_EXIT  = 2
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

func (records *Records) Ptr() uintptr {
	records_header := (*reflect.SliceHeader)(unsafe.Pointer(records))
	return records_header.Data
}

func (records *Records) AsBytes() *[]byte {
	result := new([]byte)
	records_header := (*reflect.SliceHeader)(unsafe.Pointer(records))
	result_header := (*reflect.SliceHeader)(unsafe.Pointer(result))
	result_header.Data = records_header.Data
	result_header.Len = len(*records) * int(unsafe.Sizeof((*records)[0]))
	result_header.Cap = result_header.Len
	return result
}

func (records *Records) FromBytes(bslice []byte) {
	if (len(bslice) % RecordSize()) != 0 {
		log.Panic("Unexpectedly have some bytes left over.. n=",
			(len(bslice) % RecordSize()))
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
	// TODO: Add PC.
	//       Any other useful data?
}

type MemAccess struct {
	Time             float64
	Pc, Bp, Sp, Addr uint64
	IsWrite          uint64 // because alignment.
}

func (a MemAccess) String() string {
	return fmt.Sprintf("MemAccess{t=%f write=%5t 0x%x 0x%x 0x%x 0x%x}",
		a.Time, a.IsWrite == 1, a.Pc, a.Bp, a.Sp, a.Addr)
}
