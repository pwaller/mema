package main

import (
	"image"
	"log"
	"sort"

	"github.com/JohannesEbke/go-stree/stree"
	"github.com/banthar/gl"
)

type Block struct {
	nrecords        int64
	records         Records
	context_records Records
	vertex_data     *ColorVertices

	quiet_pages, active_pages, display_active_pages map[uint64]bool
	n_pages_to_left, n_inactive_to_left             map[uint64]uint64
	stack_stree                                     *stree.Tree

	fbo *TextureBackedFBO
	img *image.RGBA

	full_data *ProgramData
	// Texture
}

func (block *Block) ActiveRegionIDs() {
	page_activity := make(map[uint64]uint)

	for i := range block.records {
		r := &block.records[i]
		if r.Type != MEMA_ACCESS {
			continue
		}
		a := r.MemAccess()
		page := a.Addr / *PAGE_SIZE

		page_activity[page]++
	}

	// Figure out how much activity the busiest page has
	var highest_page_activity uint = 0
	for _, value := range page_activity {
		if value > highest_page_activity {
			highest_page_activity = value
		}
	}

	block.quiet_pages = make(map[uint64]bool)
	block.display_active_pages = make(map[uint64]bool)
	block.active_pages = make(map[uint64]bool)
	block.n_pages_to_left = make(map[uint64]uint64)
	block.n_inactive_to_left = make(map[uint64]uint64)

	// Copy active pages from past `Nblockpast` blocks
	Nblockpast := 10
	for i := 2; i < Nblockpast; i++ {
		blocks := *&block.full_data.blocks
		j := len(blocks) - i
		if j < 0 {
			break
		}
		b := blocks[j]
		for p := range b.active_pages {
			block.display_active_pages[p] = true
		}
	}

	// Populate the "quiet pages" map (less than 1% of the activity of the 
	// most active page)
	for page, value := range page_activity {
		if *hide_qp_fraction != 0 &&
			value < highest_page_activity / *hide_qp_fraction {
			block.quiet_pages[page] = true
			continue
		}
		block.active_pages[page] = true
		block.display_active_pages[page] = true
	}

	//log.Print("Quiet pages: ", len(block.quiet_pages), " active: ", len(block.active_pages))

	// Build list of pages which are active, count active pages to the left
	active_pages := make([]uint64, len(block.display_active_pages))
	i := 0
	for page := range block.display_active_pages {
		active_pages[i] = page
		i++
	}
	sort.Sort(UInt64Slice(active_pages))

	for i := range active_pages {
		page := active_pages[i]
		block.n_pages_to_left[page] = uint64(i)
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
		block.n_inactive_to_left[p] = total_inactive_to_left
	}

	//return result
}

func (block *Block) BuildVertexData() {
	block.fbo = NewTextureBackedFBO(200, 100)

	With(Framebuffer{block.fbo}, func() {
		With(Attrib{gl.COLOR_BUFFER_BIT}, func() {
			gl.ClearColor(0, 0, 0, 1)
			gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		})
		With(Compound(Attrib{gl.VIEWPORT_BIT}, Matrix{gl.PROJECTION}), func() {
			gl.Viewport(0, 0, block.fbo.w, block.fbo.h)
			gl.LoadIdentity()
			//gl.Ortho(0, float64(fbo.w), 0, float64(fbo.h), -1, 1)
			gl.Ortho(-2, 2, 2, -2, -1, 1)

			With(Matrix{gl.MODELVIEW}, func() {
				gl.LoadIdentity()

				gl.PointSize(1)
				With(Matrix{gl.MODELVIEW}, func() {
					gl.Translated(0, -2, 0)
					gl.Scaled(1, 4/float64(block.nrecords), 1)

					block.vertex_data.Draw(gl.POINTS)
				})

				/*
					gl.Color4f(0.5, 0.5, 1, 1)
					With(Primitive{gl.LINE_LOOP}, func() {
						b := 0.2
						Squared(-2.1+b, -2.1+b, 4.35-b*2, 4.2-b*2)
					})
				*/
			})
		})
	})

	block.img = block.fbo.AsImage()
	block.vertex_data = nil
	block.records = Records{}
}

var loadingblock map[*Block]bool = make(map[*Block]bool)

func (block *Block) Draw(start, N int64) {
	if block.fbo == nil {
		if _, loading := loadingblock[block]; !loading {
			loadingblock[block] = true
			go func() {
				block.vertex_data = block.GetAccessVertexData(0, int64(block.nrecords))
				main_thread_work <- func() {
					With(&Timer{Name: "LoadTextures"}, func() {
						block.BuildVertexData()
					})
					delete(loadingblock, block)
				}
			}()
		}
		return
	}

	width := uint64(len(block.display_active_pages)) * *PAGE_SIZE
	if width == 0 {
		width = 1
	}

	gl.LineWidth(1)

	var vc, eolmarker ColorVertices

	if *pageboundaries {

		boundary_color := Color{64, 64, 64, 255}

		if width / *PAGE_SIZE < 10000 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p <= width; p += *PAGE_SIZE {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				vc.Add(ColorVertex{boundary_color, Vertex{x, 0}})
				vc.Add(ColorVertex{boundary_color, Vertex{x, float32(N)}})
			}
		}
	}

	c := Color{255, 255, 255, 255}
	eolmarker.Add(ColorVertex{c, Vertex{-2, 0}})
	eolmarker.Add(ColorVertex{c, Vertex{2, 0}})

	gl.LineWidth(1)
	With(&Timer{Name: "DrawPartial"}, func() {
		var x1, y1, x2, y2 float64
		With(Matrix{gl.MODELVIEW}, func() {
			gl.Translated(0, -2, 0)
			gl.Scaled(1, 4/float64(*nback), 1)
			gl.Translated(0, -float64(start), 0)

			//gl.Color4f(0.5, 0, 0, 0.2)
			//DrawQuadi(-2, 0, 4, int(N))

			gl.PointSize(2)

			//block.vertex_data.Draw(gl.POINTS)

			x1, y1 = ProjToWindow(-2, 0)
			x2, y2 = ProjToWindow(-2+4, float64(N))

			//_, h := GetViewportWH()
			//log.Printf("  h = %f -- %f", y1-y2, float64(int64(h)*N/(*nback))/(2.25*2- -2.1*2)*4)

		})
		With(WindowCoords{}, func() {
			gl.Color4f(1, 1, 1, 1)
			With(Texture{block.fbo.texture, gl.TEXTURE_2D}, func() {
				DrawQuadd(x1, y1, x2-x1, y2-y1)
			})

			/*
				gl.Color4f(1, 1, 1, 1)
				With(Primitive{gl.LINE_LOOP}, func() {
					Squared(640+x1, y1, x2-x1, y2-y1)
				})
			*/
		})
		With(Matrix{gl.MODELVIEW}, func() {
			gl.Translated(0, -2, 0)
			gl.Scaled(1, 4/float64(*nback), 1)
			gl.Translated(0, -float64(start), 0)

			With(Attrib{gl.ENABLE_BIT}, func() {
				gl.Disable(gl.LINE_SMOOTH)
				vc.Draw(gl.LINES)
				//eolmarker.Draw(gl.LINES)
			})
		})
	})
}

func (block *Block) GetAccessVertexData(start, N int64) *ColorVertices {

	width := uint64(len(block.display_active_pages)+1) * *PAGE_SIZE

	vc := &ColorVertices{}

	// TODO: Transport vertices to the GPU in bulk using glBufferData
	//	   Function calls here appear to be the biggest bottleneck
	// 		OTOH, this might not be supported on older cards
	var stack_depth int = len(block.context_records)

	for pos := start; pos < min(start+N, int64(block.nrecords)); pos++ {
		if pos < 0 {
			continue
		}

		r := &block.records[pos]
		if r.Type == MEMA_ACCESS {
			// take it
		} else if r.Type == MEMA_FUNC_ENTER {
			stack_depth++

			y := float32(int64(len(*vc)) - start)
			c := Color{64, 64, 255, 255}
			vc.Add(ColorVertex{c, Vertex{2 + float32(stack_depth)/80., y}})

			continue
		} else if r.Type == MEMA_FUNC_EXIT {

			y := float32(int64(len(*vc)) - start)
			c := Color{255, 64, 64, 255}
			vc.Add(ColorVertex{c, Vertex{2 + float32(stack_depth)/80., y}})

			stack_depth--

			continue
		} else {
			log.Panic("Unexpected record type: ", r.Type)
		}
		a := r.MemAccess()

		page := a.Addr / *PAGE_SIZE
		if _, present := block.quiet_pages[page]; present {
			continue
		}

		x := float32((a.Addr - block.n_inactive_to_left[page]*(*PAGE_SIZE))) / float32(width)
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
