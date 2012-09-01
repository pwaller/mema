package main

import (
	"log"
	"sort"
	
	"github.com/toberndo/go-stree/stree"
)

func (d *ProgramData) BuildStree() *stree.Tree {
	tree := stree.NewTree()
	s := new(Stack)
	
	for i := range d.records {
		r := &d.records[i]
		if r.Type == MEMA_FUNC_ENTER {
			s.Push(i)
		} else if r.Type == MEMA_FUNC_EXIT {
			i_start := s.Pop().(int)
			// These should match, otherwise we're looking at something incomplete
			if d.records[i_start].FunctionCall().FuncPointer != 
			   d.records[i].FunctionCall().FuncPointer {
			   	log.Panic("Not matching.. - ", d.records[i_start].FunctionCall().FuncPointer,
			   		" - ", d.records[i].FunctionCall().FuncPointer, " ", i_start, " ", i)
			}
			if s.size != 0 {
				tree.Push(i_start, i-1)
			}
		}
	}
	
	tree.BuildTree()
	
	return &tree
}

// Returns the stack frame for a given record id
func (d *ProgramData) GetStack(record int64) []*Record {
	intervals := (*d.stack_stree).Query(int(record), int(record))
	entry_indices := make([]int, len(intervals))
	for i := range intervals {
		entry_indices[i] = intervals[i].Segment.From
	}
	sort.Ints(entry_indices)
	result := make([]*Record, len(intervals))
	for i := range entry_indices {
		result[i] = &d.records[entry_indices[i]]
	}
	
	return result
}

func (d *ProgramData) GetStackNames(record int64) []string {
	stack := d.GetStack(record)
	result := make([]string, len(stack))
	for i := range stack {
		f := stack[i].FunctionCall()
		region := d.GetRegion(f.FuncPointer)
		result[i] = region.GetSymbol(f.FuncPointer)
	}
	return result
}
