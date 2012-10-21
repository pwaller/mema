package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"time"

	"github.com/pwaller/go-hexcolor"
	"github.com/pwaller/go-memhelper"

	"github.com/ajstarks/svgo"
	"github.com/vdobler/chart"
	"github.com/vdobler/chart/svgg"

	"github.com/go-gl/gl"
	"github.com/go-gl/glchart"
	"github.com/go-gl/gldebug/gpuinfo"
	"github.com/go-gl/glh"
)

// type StatusHUDValue struct {
// 	sync.RWMutex
// 	Value interface{}
// }

// func (v *StatusHUDValue) Update(newvalue interface{}) {
// 	v.Lock()
// 	defer v.Unlock()
// 	v.Value = newvalue
// }

// type StatusHUDValues []StatusHUDValue

// var Values = StatusHUDValues{}

// func (StatusHUDValues)
type Statistic struct {
	Samples *[]chart.EPoint
	Eval    func() float64
}
type Statistics []Statistic

func (s *Statistics) Add(c *chart.ScatterChart, name, col string, eval func() float64) {
	xs, ys := []float64{}, []float64{}

	rgbac := color.RGBAModel.Convert(hexcolor.Hex(col))

	c.AddDataPair(name, xs, ys, chart.PlotStyleLines,
		chart.Style{LineColor: rgbac, LineWidth: 1})

	*s = append(*s, Statistic{Eval: eval})
	for i := range c.Data {
		(*s)[i].Samples = &c.Data[i].Samples
	}
}

func (s *Statistics) Update() float64 {
	now := time.Now()
	max := 0.
	for _, stat := range *s {
		value := stat.Eval()
		if value > max {
			max = value
		}
		p := chart.EPoint{X: float64(now.UnixNano()),
			Y: value, DeltaX: math.NaN(), DeltaY: math.NaN()}
		/*
			if len(*stat.Samples) > 1024 {
				const (
					STRIDE      = 8
					KEEP_AMOUNT = 256
				)
				keep := make([]chart.EPoint, KEEP_AMOUNT/STRIDE)
				for i := 0; i < KEEP_AMOUNT; i += STRIDE {
					keep[i/STRIDE] = (*stat.Samples)[i]
				}
				*stat.Samples = append(keep, (*stat.Samples)[KEEP_AMOUNT:]...)
			}
		*/
		*stat.Samples = append(*stat.Samples, p)
	}
	return max
}

var StatsHUD = func() {}
var DumpStatsHUD = func() {}

func InitStatsHUD() {
	plots := chart.ScatterChart{Title: "", Options: glchart.DarkStyle}
	start := time.Now()

	l := float64(start.UnixNano())
	r := float64(start.Add(2 * time.Second).UnixNano())

	plots.XRange.Fixed(l, r, 1e9)
	plots.YRange.Fixed(0.1, 100, 10)

	plots.XRange.TicSetting.Tics, plots.YRange.TicSetting.Tics = 1, 1
	plots.XRange.TicSetting.Mirror, plots.YRange.TicSetting.Mirror = 2, 2
	plots.XRange.TicSetting.Grid, plots.YRange.TicSetting.Grid = 2, 2

	plots.YRange.ShowZero = true

	//plots.XRange.Log = true
	//plots.YRange.Log = true

	plots.Key.Pos, plots.Key.Cols = "obc", 3

	plots.XRange.TicSetting.Format = func(f float64) string {
		t := time.Unix(int64(f)/1e9, int64(f)%1e9)
		return fmt.Sprintf("%.3v", time.Since(t))
	}

	memhelper.GetMaxRSS()

	var gpufree float64

	gpupoll := gpuinfo.PollGPUMemory()
	go func() {
		for gpustatus := range gpupoll {
			gpufree = float64(memhelper.ByteSize(gpustatus.Free()) * memhelper.MiB)
		}
	}()

	statistics := &Statistics{}
	statistics.Add(&plots, "GPU Free", "#FF9F00", func() float64 { return gpufree })
	statistics.Add(&plots, "SpareRAM()", "#ff0000", func() float64 { return float64(SpareRAM() * 1e6) })
	statistics.Add(&plots, "MaxRSS", "#FFE240", func() float64 { return float64(memhelper.GetMaxRSS()) })
	statistics.Add(&plots, "Heap Idle", "#33ff33", func() float64 { return float64(memstats.HeapIdle) })
	statistics.Add(&plots, "Alloc", "#FF6600", func() float64 { return float64(memstats.Alloc) })
	statistics.Add(&plots, "Heap Alloc", "#006699", func() float64 { return float64(memstats.HeapAlloc) })
	statistics.Add(&plots, "Sys", "#996699", func() float64 { return float64(memstats.Sys) })
	statistics.Add(&plots, "System Free", "#3333ff", func() float64 { return float64(SystemFree()) })
	statistics.Add(&plots, "nBlocks x 1e6", "#FFCC00", func() float64 { return float64(nblocks * 1e6) })
	statistics.Add(&plots, "nDrawn x 1e6", "#9C8AA5", func() float64 { return float64(blocks_rendered * 1e6) })

	go func() {
		top := 0.

		i := -1
		for {
			time.Sleep(250 * time.Millisecond)
			max := statistics.Update()
			if max > top {
				top = max
			}
			i++
			if i%4 != 0 {
				continue
			}

			segment := float64(1e9)
			if time.Since(start) > 10*time.Second {
				segment = 5e9
			}
			if time.Since(start) > 1*time.Minute {
				segment = 30e9
			}

			// Update axis limits
			nr := float64(time.Now().Add(2 * time.Second).UnixNano())
			plots.XRange.Fixed(l, nr, segment)
			plots.YRange.Fixed(-1e9, top*1.1, 500e6)
		}
	}()

	const pw, ph = 640, 480

	scalex, scaley := 0.4, 0.5

	chart_gfxcontext := glchart.New(pw, ph, "", 10, color.RGBA{})

	StatsHUD = func() {
		glh.With(glh.Matrix{gl.PROJECTION}, func() {
			gl.LoadIdentity()
			gl.Translated(1-scalex, scaley-1, 0)
			gl.Scaled(scalex, scaley, 1)
			gl.Ortho(0, pw, ph, 0, -1, 1)
			gl.Translated(0, -50, 0)

			glh.With(glh.Attrib{gl.ENABLE_BIT}, func() {
				gl.Disable(gl.DEPTH_TEST)
				glh.With(&Timer{Name: "Chart"}, func() {
					plots.Plot(chart_gfxcontext)
				})
			})
		})
	}

	// TODO: figure out why this is broken
	DumpStatsHUD = func() {
		s2f, _ := os.Create("statshud-dump.svg")
		mysvg := svg.New(s2f)
		mysvg.Start(1600, 800)
		mysvg.Rect(0, 0, 2000, 800, "fill: #ffffff")
		sgr := svgg.New(mysvg, 2000, 800, "Arial", 18,
			color.RGBA{0xff, 0xff, 0xff, 0xff})
		sgr.Begin()

		plots.Plot(sgr)

		sgr.End()
		mysvg.End()
		s2f.Close()
		log.Print("Saved statshud-dump.svg")
	}

	//log.Print("InitStatsHUD()")
}
