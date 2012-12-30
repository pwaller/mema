package main

import (
	// "bytes"
	"encoding/gob"

// "io"
// "log"
// "os"

// "github.com/jteeuwen/magic"
// "github.com/pwaller/go-clz4"
)

type PackedBinary struct {
	CompressedData []byte
}

func (pb *PackedBinary) Decode(decoder *gob.Decoder) {
	decoder.Decode(pb)
}

var allowed_filetypes = map[string]bool{
	"application/x-sharedlib":  true,
	"application/x-executable": true,
}

func (data *ProgramData) PackBinaries() {

	// files := make(map[string]bool)

	// for i := range data.region {
	// 	files[data.region[i].pathname] = true
	// }

	// total := int64(0)
	// totalw := int64(0)

	// file_data := new(bytes.Buffer)
	// compressed_data := new(bytes.Buffer)
	// compressed_bytes := []byte{}

	// magic_db, err := magic.Open(magic.FlagMimeType)
	// defer magic_db.Close()
	// if err != nil {
	// 	log.Panicf("Failed to open magic db: %v", err)
	// }
	// err = magic_db.Load("")
	// if err != nil {
	// 	log.Panicf("Failed to load magic db: %v", err)
	// }

	// for k, _ := range files {
	// 	fi, err := os.Stat(k)
	// 	if err != nil {
	// 		continue
	// 	}

	// 	t, err := magic_db.File(k)
	// 	if _, ok := allowed_filetypes[t]; !ok {
	// 		log.Print("Skipping disallowed file: %v", k)
	// 		continue
	// 	}

	// 	fd, err := os.Open(k)
	// 	log.Printf("File: '%s': %+v", k, fi)
	// 	total += fi.Size()
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	defer fd.Close()
	// 	w, err := io.Copy(file_data, fd)
	// 	if err != nil {
	// 		log.Fatal("Error copying bytes from fd to wcd: ", err, " tot = ", totalw, ", ", w)
	// 	}
	// 	clz4.Compress(file_data.Bytes(), &compressed_bytes)
	// 	compressed_data.Write(compressed_bytes)
	// 	file_data.Reset()

	// 	totalw += w
	// }

	// log.Print("Total size of binaries: ", total)
	// log.Print("Total written: ", totalw)
	// log.Print("Total compressed size of binaries: ", compressed_data.Len())
}

func (data *ProgramData) LoadBinaryPack() {

}
