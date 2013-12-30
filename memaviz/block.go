package main

import (
	"image"
	"image/color"
	"log"
	"runtime"
	"sort"
	"sync"

	"github.com/JohannesEbke/go-stree/stree"

	"github.com/go-gl/gl"
	"github.com/go-gl/glh"
)

type Block struct {
	nrecords        int64
	detail_needed   bool
	records         Records
	context_records Records
	vertex_data     *glh.MeshBuffer

	quiet_pages, active_pages, display_active_pages map[uint64]bool
	n_pages_to_left, n_inactive_to_left             map[uint64]uint64
	stack_stree                                     *stree.Tree

	tex *glh.Texture
	img *image.RGBA

	full_data   *ProgramData
	file_offset int64

	requests struct {
		texture, vertices sync.Once
	}
	// Texture
}

const WIDTH = 4.25

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
	Nblockpast := 0
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
}

func (block *Block) BuildTexture() {
	block.tex = glh.NewTexture(1024, 256)
	block.tex.Init()

	// TODO: use runtime.SetFinalizer() to clean up/delete the texture?
	glh.With(block.tex, func() {
		//gl.TexParameteri(gl.TEXTURE_2D, gl.GENERATE_MIPMAP, gl.TRUE)

		//gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)

		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST_MIPMAP_NEAREST)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		// TODO: Only try and activate anisotropic filtering if it is available

		gl.TexParameterf(gl.TEXTURE_2D, GL_TEXTURE_MAX_ANISOTROPY_EXT, MaxAnisotropy)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAX_LEVEL, 1)
	})

	for i := 0; i < 2; i++ {
		glh.With(&glh.Framebuffer{Texture: block.tex, Level: i}, func() {
			glh.With(glh.Attrib{gl.COLOR_BUFFER_BIT}, func() {
				gl.ClearColor(1, 0, 0, 0)
				gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
			})
			viewport_proj := glh.Compound(glh.Attrib{gl.VIEWPORT_BIT},
				glh.Matrix{gl.PROJECTION})

			glh.With(viewport_proj, func() {
				gl.Viewport(0, 0, block.tex.W/(1<<uint(i)), block.tex.H/(1<<uint(i)))
				gl.LoadIdentity()
				//gl.Ortho(0, float64(tex.w), 0, float64(tex.h), -1, 1)
				gl.Ortho(-2, -2+WIDTH, 2, -2, -1, 1)

				glh.With(glh.Matrix{gl.MODELVIEW}, func() {
					gl.LoadIdentity()

					//gl.Hint(gl.LINES, gl.NICEST)
					//gl.LineWidth(4)
					/*
						glh.With(glh.Primitive{gl.LINES}, func() {
							gl.Color4f(1, 1, 1, 1)
							gl.Vertex2f(-2, 0)
							gl.Vertex2f(2, 0)
						})
					*/

					gl.PointSize(4)
					glh.With(glh.Matrix{gl.MODELVIEW}, func() {
						gl.Translated(0, -2, 0)
						gl.Scaled(1, 4/float64(block.nrecords), 1)

						block.vertex_data.Render(gl.POINTS)
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
	}

	//block.img = block.tex.AsImage()
	if !block.detail_needed {
		block.vertex_data = nil
		runtime.GC()
	}

	blocks_rendered++
}

var blocks_rendered = int64(0)

//var loading_texture map[*Block]bool = make(map[*Block]bool)

func (block *Block) RequestTexture() {
	block.requests.texture.Do(func() {
		// Schedule the main thread to build our texture for us
		go func() {
			main_thread_work <- func() {
				glh.With(&Timer{Name: "LoadTextures"}, func() {
					block.BuildTexture()
				})
				// Allow generation once again
				block.requests.texture = sync.Once{}
			}
		}()
	})
}

func (block *Block) RequestVertices() {
	block.requests.vertices.Do(func() {
		// This request is processed by the file reading go-routine
		block.full_data.detail_request <- block
	})
}

func (block *Block) Draw(start, N int64, detailed bool) {
	if block.tex == nil {
		block.RequestTexture()
	}

	switch detailed {
	case true:
		block.detail_needed = true
		if block.vertex_data == nil {
			// Hey, we need vertices but don't have them! Let's fix that..
			block.RequestVertices()
		}
	default:
		block.detail_needed = false
	}

	width := uint64(len(block.display_active_pages)) * *PAGE_SIZE
	if width == 0 {
		width = 1
	}

	var vc glh.ColorVertices

	if *pageboundaries {
		boundary_color := color.RGBA{64, 64, 64, 255}

		if width / *PAGE_SIZE < 10000 { // If we try and draw too many of these, X will hang
			for p := uint64(0); p <= width; p += *PAGE_SIZE {
				x := float32(p) / float32(width)
				x = (x - 0.5) * 4
				vc.Add(glh.ColorVertex{boundary_color, glh.Vertex{x, 0}})
				vc.Add(glh.ColorVertex{boundary_color, glh.Vertex{x, float32(N)}})
			}
		}
	}

	var border_color [4]float64

	gl.LineWidth(1)
	glh.With(&Timer{Name: "DrawPartial"}, func() {
		var x1, y1, x2, y2 float64
		glh.With(glh.Matrix{gl.MODELVIEW}, func() {
			// TODO: A little less co-ordinate insanity?
			gl.Translated(0, -2, 0)
			gl.Scaled(1, 4/float64(*nback), 1)
			gl.Translated(0, -float64(start), 0)

			x1, y1 = glh.ProjToWindow(-2, 0)
			x2, y2 = glh.ProjToWindow(-2+WIDTH, float64(N))

		})
		border_color = [4]float64{1, 1, 1, 1}

		glh.With(glh.Matrix{gl.MODELVIEW}, func() {
			gl.Translated(0, -2, 0)
			gl.Scaled(1, 4/float64(*nback), 1)
			gl.Translated(0, -float64(start), 0)

			// Page boundaries
			// TODO: Use different blending scheme on textured quads so that the
			//       lines show through
			glh.With(glh.Attrib{gl.ENABLE_BIT}, func() {
				gl.Disable(gl.LINE_SMOOTH)
				vc.Draw(gl.LINES)
			})
		})

		if block.tex != nil && (!detailed || block.vertex_data == nil) {
			border_color = [4]float64{0, 0, 1, 1}
			glh.With(glh.WindowCoords{Invert: true}, func() {
				gl.Color4f(1, 1, 1, 1)
				// Render textured block quad
				glh.With(block.tex, func() {
					glh.DrawQuadd(x1, y1, x2-x1, y2-y1)
				})
				glh.With(glh.Primitive{gl.LINES}, func() {
					glh.Squared(x1, y1, x2-x1, y2-y1)
				})
			})
			if block.vertex_data != nil && !block.detail_needed {
				// TODO: figure out when we can unload
				// Hey, we can unload you, because you are not needed
				block.vertex_data = nil
			}

		}
		if detailed && block.vertex_data != nil {
			glh.With(glh.Matrix{gl.MODELVIEW}, func() {
				// TODO: A little less co-ordinate insanity?
				gl.Translated(0, -2, 0)
				gl.Scaled(1, 4/float64(*nback), 1)
				gl.Translated(0, -float64(start), 0)

				gl.PointSize(2)
				block.vertex_data.Render(gl.POINTS)
			})
		}

		glh.With(glh.WindowCoords{Invert: true}, func() {
			// Block boundaries
			gl.Color4dv(&border_color)

			gl.LineWidth(1)
			glh.With(glh.Primitive{gl.LINE_LOOP}, func() {
				glh.Squared(x1, y1, x2-x1, y2-y1)
			})
		})
	})
}

func (block *Block) GenerateVertices() *glh.MeshBuffer {

	width := uint64(len(block.display_active_pages)+1) * *PAGE_SIZE

	vc := glh.NewMeshBuffer(
		glh.RenderArrays,
		glh.NewPositionAttr(2, gl.FLOAT, gl.STATIC_DRAW),
		glh.NewColorAttr(3, gl.UNSIGNED_BYTE, gl.STATIC_DRAW),
	)

	var stack_depth int = len(block.context_records)

	vertex := []float32{0, 0}
	colour := []uint8{0, 0, 0}
	x, y := &vertex[0], &vertex[1]
	r, g, b := &colour[0], &colour[1], &colour[2]

	vertices := make([]float32, 0, block.nrecords*2)
	colours := make([]uint8, 0, block.nrecords*3)

	for pos := int64(0); pos < int64(block.nrecords); pos++ {
		if pos < 0 {
			continue
		}

		rec := &block.records[pos]
		if rec.Type == MEMA_ACCESS {
			// take it
		} else if rec.Type == MEMA_FUNC_ENTER {
			stack_depth++

			*x = 2 + float32(stack_depth)/80.
			*y = float32(pos) //int64(len(*vc)))
			*r, *g, *b = 64, 64, 255

			vertices = append(vertices, *x, *y)
			colours = append(colours, *r, *g, *b)
			//vc.Add(vertex, colour)
			//c := color.RGBA{64, 64, 255, 255}
			//vc.Add(glh.ColorVertex{c, glh.Vertex{, y}})

			continue
		} else if rec.Type == MEMA_FUNC_EXIT {

			*x = 2 + float32(stack_depth)/80.
			*y = float32(pos) //int64(len(*vc)))
			*r, *g, *b = 255, 64, 64
			vertices = append(vertices, *x, *y)
			colours = append(colours, *r, *g, *b)
			//vc.Add(vertex, colour)
			//y := float32(int64(len(*vc)))
			//c := color.RGBA{255, 64, 64, 255}
			//vc.Add(glh.ColorVertex{c, glh.Vertex{2 + float32(stack_depth)/80., y}})

			stack_depth--

			continue
		} else {
			log.Panic("Unexpected record type: ", rec.Type)
		}
		a := rec.MemAccess()

		page := a.Addr / *PAGE_SIZE
		if _, present := block.quiet_pages[page]; present {
			continue
		}

		*x = float32((a.Addr - block.n_inactive_to_left[page]*(*PAGE_SIZE))) / float32(width)
		*x = (*x - 0.5) * 4

		if *x > 4 || *x < -4 {
			log.Panic("x has unexpected value: ", x)
		}

		*y = float32(pos) //len(*vc))
		*r, *g, *b = uint8(a.IsWrite)*255, uint8(1-a.IsWrite)*255, 0

		vertices = append(vertices, *x, *y)
		colours = append(colours, *r, *g, *b)

		//vc.Add(vertex, colour)

		//c := color.RGBA{uint8(a.IsWrite) * 255, uint8(1-a.IsWrite) * 255, 0, 255}

		//vc.Add(glh.ColorVertex{c, glh.Vertex{x, y}})

		/*
			TODO: Reintroduce 'recently hit memory locations'
			if pos > (start + N) - N / 20 {
				vc.Add(ColorVertex{c, Vertex{x, 2 + 0.1}})
				vc.Add(ColorVertex{c, Vertex{x, 2}})
			}
		*/
	}

	vc.Add(vertices, colours)
	// Don't need the record data anymore
	// TOOD(pwaller): figure this out
	// block.records = Records{}
	runtime.GC()

	return vc
}
