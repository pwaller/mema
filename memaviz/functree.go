// functree.go: Building a tree to efficiently determine a callstack given a
// 			 	record index

package main

import (
	"log"
	"sort"

	"github.com/JohannesEbke/go-stree/stree"
)

func (b *Block) BuildStree() (*stree.Tree, Records) {
	tree := stree.NewTree()
	s := new(Stack)

	for i := range b.context_records {
		s.Push(-len(b.context_records) + i)
	}

	for i := range b.records {
		r := &b.records[i]
		if r.Type == MEMA_FUNC_ENTER {
			s.Push(i)
		} else if r.Type == MEMA_FUNC_EXIT {
			i_start := s.Pop().(int)
			var r *Record
			if i_start < 0 {
				r = &b.context_records[-i_start-1]
			} else {
				r = &b.records[i_start]
			}
			// These should match, otherwise we're looking at something incomplete
			if (*r).FunctionCall().FuncPointer != (*r).FunctionCall().FuncPointer {
				log.Panic("Not matching.. - ", (*r).FunctionCall().FuncPointer,
					" - ", (*r).FunctionCall().FuncPointer, " ", i_start, " ", i)
			}
			tree.Push(i_start, i-1)
		}
	}

	return_context := make(Records, s.size)
	i := 0
	for s.size > 0 {
		i_start := s.Pop().(int)
		tree.Push(i_start, len(b.records))
		if i_start < 0 {
			//log.Print("CR: ", len(b.context_records), -i_start)
			return_context[i] = b.context_records[-i_start-1]
		} else {
			return_context[i] = b.records[i_start]
		}
		i += 1
	}

	tree.BuildTree()

	return &tree, return_context
}

// Returns the stack frame for a given record id
func (block *Block) GetStack(record int64) []*Record {
	if block.stack_stree == nil {
		return []*Record{}
	}
	intervals := (*block.stack_stree).Query(int(record), int(record))
	entry_indices := make([]int, len(intervals))
	for i := range intervals {
		entry_indices[i] = intervals[i].Segment.From
	}
	sort.Ints(entry_indices)
	result := make([]*Record, len(intervals))
	for i := range entry_indices {
		if entry_indices[i] < 0 {
			result[i] = &block.context_records[-entry_indices[i]-1]
		} else {
			result[i] = &block.records[entry_indices[i]]
		}
	}

	return result
}

func (block *Block) GetStackNames(record int64) []string {
	stack := block.GetStack(record)
	result := make([]string, len(stack))
	for i := range stack {
		f := stack[i].FunctionCall()
		result[i] = block.GetSymbol(f.FuncPointer)
	}
	return result
}
