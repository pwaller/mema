package main

import (
	"sort"
)

type UInt64Slice []uint64

func (p UInt64Slice) Len() int			 { return len(p) }
func (p UInt64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p UInt64Slice) Swap(i, j int)		 { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.
func (p UInt64Slice) Sort() { sort.Sort(p) }

func min(a, b int64) int64 { if b < a { return b }; return a }
