package main

import (
	"log"
	"sort"

	"github.com/banthar/gl"
	"github.com/toberndo/go-stree/stree"
)

type Block struct {
	nrecords    int64
	records     Records
	vertex_data []*ColorVertices

	quiet_pages        map[uint64]bool
	active_pages       map[uint64]bool
	n_pages_to_left    map[uint64]uint64
	n_inactive_to_left map[uint64]uint64
	stack_stree        *stree.Tree

	full_data *ProgramData
	// Texture
}

func (d *Block) ActiveRegionIDs() []int {
	// TODO: Quite a bite of this wants to be moved into NewProgramData
	active := make(map[int]bool)
	page_activity := make(map[uint64]uint)

	for i := range d.records {
		r := &d.records[i]
		if r.Type != MEMA_ACCESS {
			continue
		}
		a := r.MemAccess()
		active[d.full_data.RegionID(a.Addr)] = true
		page := a.Addr / *PAGE_SIZE

		page_activity[page]++
	}
	result := make([]int, 0)
	for k := range active {
		result = append(result, k)
	}
	sort.Ints(result)

	// Figure out how much activity the busiest page has
	var highest_page_activity uint = 0
	for _, value := range page_activity {
		if value > highest_page_activity {
			highest_page_activity = value
		}
	}

	d.quiet_pages = make(map[uint64]bool)
	d.active_pages = make(map[uint64]bool)
	d.n_pages_to_left = make(map[uint64]uint64)
	d.n_inactive_to_left = make(map[uint64]uint64)

	// Populate the "quiet pages" map (less than 1% of the activity of the 
	// most active page)
	for page, value := range page_activity {
		if *hide_qp_fraction != 0 &&
			value < highest_page_activity / *hide_qp_fraction {
			d.quiet_pages[page] = true
			continue
		}
		d.active_pages[page] = true
	}

	log.Print("Quiet pages: ", len(d.quiet_pages), " active: ", len(d.active_pages))

	// Build list of pages which are active, count active pages to the left
	active_pages := make([]uint64, len(d.active_pages))
	i := 0
	for page := range d.active_pages {
		active_pages[i] = page
		i++
	}
	sort.Sort(UInt64Slice(active_pages))

	for i := range active_pages {
		page := active_pages[i]
		d.n_pages_to_left[page] = uint64(i)
	}

	var total_inactive_to_left uint64 = 0
	for i = range active_pages {
		p := active_pages[i]
		var inactive_to_left uint64
		if i == 0 {
			inactive_to_left = active_pages[i]
		} else {
			inactive_to_left = active_pages[i] - active_pages[i-1]
		}
		total_inactive_to_left += inactive_to_left - 1
		d.n_inactive_to_left[p] = total_inactive_to_left
	}

	return result
}

func (data *Block) Draw(start, N int64) bool {
	defer OpenGLSentinel()()

	width := uint64(len(data.active_pages)) * *PAGE_SIZE
	if width == 0 {
		width = 1
	}

	gl.LineWidth(1)

	if *pageboundaries {

		var vc ColorVertices
		boundary_color := Color{64, 64, 64, 255}

		if width / *PAGE_SIZE < 1000 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p < width; p += *PAGE_SIZE {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				vc.Add(ColorVertex{boundary_color, Vertex{x, -2}})
				vc.Add(ColorVertex{boundary_color, Vertex{x, 2}})
			}
		}

		With(Attrib{gl.ENABLE_BIT}, func() {
			gl.Disable(gl.LINE_SMOOTH)
			vc.Draw()
		})
	}

	gl.LineWidth(1)

	if cap(data.vertex_data) == 0 {
		log.Print("start = ", start)
		data.vertex_data = make([]*ColorVertices, 1)
		data.vertex_data[0] = data.GetAccessVertexData(0, int64(data.nrecords))
	}

	With(Matrix{gl.MODELVIEW}, func() {
		gl.Translated(0, -2, 0)
		gl.Scaled(1, 4/float64(*nback), 1)
		gl.Translated(0, -float64(start), 0)

		data.vertex_data[0].DrawPartial(start, *nback)
	})

	NV := int64(len(*data.vertex_data[0]))

	var eolmarker ColorVertices

	// End-of-data marker
	if start+N > NV {
		y := -2 + 4*float32(NV-start)/float32(*nback)
		c := Color{255, 255, 255, 255}
		eolmarker.Add(ColorVertex{c, Vertex{-2, y}})
		eolmarker.Add(ColorVertex{c, Vertex{2, y}})
	}

	OpenGLSentinel()

	gl.LineWidth(1)
	eolmarker.Draw()

	return start > NV //int64(data.nrecords)
}

func (data *Block) GetAccessVertexData(start, N int64) *ColorVertices {

	width := uint64(len(data.active_pages)) * *PAGE_SIZE

	vc := &ColorVertices{}

	// TODO: Transport vertices to the GPU in bulk using glBufferData
	//	   Function calls here appear to be the biggest bottleneck
	// 		OTOH, this might not be supported on older cards
	var stack_depth int

	for pos := start; pos < min(start+N, int64(data.nrecords)); pos++ {
		if pos < 0 {
			continue
		}

		r := &data.records[pos]
		if r.Type == MEMA_ACCESS {
			// take it
		} else if r.Type == MEMA_FUNC_ENTER {
			stack_depth++

			y := float32(int64(len(*vc)) - start)
			c := Color{64, 64, 255, 255}
			vc.Add(ColorVertex{c, Vertex{2.1 + float32(stack_depth)/80., y}})

			continue
		} else if r.Type == MEMA_FUNC_EXIT {

			y := float32(int64(len(*vc)) - start)
			c := Color{255, 64, 64, 255}
			vc.Add(ColorVertex{c, Vertex{2.1 + float32(stack_depth)/80., y}})

			stack_depth--

			continue
		} else {
			log.Panic("Unexpected record type: ", r.Type)
		}
		a := r.MemAccess()

		page := a.Addr / *PAGE_SIZE
		if _, present := data.quiet_pages[page]; present {
			continue
		}

		x := float32((a.Addr - data.n_inactive_to_left[page]*(*PAGE_SIZE))) / float32(width)
		x = (x - 0.5) * 4

		if x > 4 || x < -4 {
			log.Panic("x has unexpected value: ", x)
		}

		y := float32(int64(len(*vc)) - start)

		c := Color{uint8(a.IsWrite) * 255, uint8(1-a.IsWrite) * 255, 0, 255}

		vc.Add(ColorVertex{c, Vertex{x, y}})

		/*
			TODO: Reintroduce 'recently hit memory locations'
			if pos > (start + N) - N / 20 {
				vc.Add(ColorVertex{c, Vertex{x, 2 + 0.1}})
				vc.Add(ColorVertex{c, Vertex{x, 2}})
			}
		*/
	}

	return vc
}