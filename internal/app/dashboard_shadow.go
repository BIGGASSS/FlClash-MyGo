package app

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/vector"
)

type startShadowKey struct {
	side            int
	progress, color uint32
}
type startShadowEntry struct {
	bitmap *ui.Bitmap
	used   uint64
}

var startShadows = map[startShadowKey]startShadowEntry{}
var startShadowUse uint64

// The source blurs the morph's alpha mask, including its concave edges.
// Rasterize that small native vector mask at 2× and cache it, rather than
// substituting a rectangular/circular CSS shadow for the cookie outline.
func startShadow(side, progress float32, color ui.Color) *ui.Bitmap {
	startShadowUse++
	key := startShadowKey{int(math.Round(float64(side) * 2)), math.Float32bits(progress), uint32(color.R)<<24 | uint32(color.G)<<16 | uint32(color.B)<<8 | uint32(color.A)}
	if entry, ok := startShadows[key]; ok {
		entry.used = startShadowUse
		startShadows[key] = entry
		return entry.bitmap
	}
	const margin = 28
	size := key.side + margin*4
	mask := image.NewAlpha(image.Rect(0, 0, size, size))
	raster := vector.NewRasterizer(size, size)
	dashboardShapesOnce.Do(loadDashboardShapes)
	sin, cos := float32(math.Sin(float64(progress)*2*math.Pi/9)), float32(math.Cos(float64(progress)*2*math.Pi/9))
	point := func(x, y float32) (float32, float32) {
		x, y = x-0.5, y-0.5
		// The painter translates its shadow after rotating the unit shape.
		y += 6 / side
		return float32(size)/2 + (x*cos-y*sin)*float32(key.side), float32(size)/2 + (x*sin+y*cos)*float32(key.side)
	}
	for i, curve := range dashboardShapes["start0"] {
		end := dashboardShapes["start1"][i]
		for j := range curve {
			curve[j] += (end[j] - curve[j]) * progress
		}
		if i == 0 {
			x, y := point(curve[0], curve[1])
			raster.MoveTo(x, y)
		}
		x1, y1 := point(curve[2], curve[3])
		x2, y2 := point(curve[4], curve[5])
		x3, y3 := point(curve[6], curve[7])
		raster.CubeTo(x1, y1, x2, y2, x3, y3)
	}
	raster.ClosePath()
	raster.Draw(mask, mask.Bounds(), image.White, image.Point{})
	// Flutter's mask sigma is shadowBlur/2 = 7 logical pixels.
	const sigma = 14.0
	const radius = 42
	var kernel [radius*2 + 1]float32
	weight := float32(0)
	for i := range kernel {
		d := float64(i - radius)
		kernel[i] = float32(math.Exp(-d * d / (2 * sigma * sigma)))
		weight += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= weight
	}
	horizontal := make([]float32, size*size)
	for y := range size {
		for x := range size {
			value := float32(0)
			for d := -radius; d <= radius; d++ {
				if xx := x + d; xx >= 0 && xx < size {
					value += float32(mask.Pix[y*mask.Stride+xx]) * kernel[d+radius]
				}
			}
			horizontal[y*size+x] = value
		}
	}
	pixels := image.NewNRGBA(mask.Bounds())
	for y := range size {
		for x := range size {
			value := float32(0)
			for d := -radius; d <= radius; d++ {
				if yy := y + d; yy >= 0 && yy < size {
					value += horizontal[yy*size+x] * kernel[d+radius]
				}
			}
			at := y*pixels.Stride + x*4
			pixels.Pix[at], pixels.Pix[at+1], pixels.Pix[at+2], pixels.Pix[at+3] = color.R, color.G, color.B, uint8(math.Round(float64(value*float32(color.A)/255*0.3)))
		}
	}
	bitmap := ui.NewBitmap(pixels)
	if len(startShadows) >= 32 {
		var oldest startShadowKey
		age := uint64(math.MaxUint64)
		for key, entry := range startShadows {
			if entry.used < age {
				oldest, age = key, entry.used
			}
		}
		delete(startShadows, oldest)
	}
	startShadows[key] = startShadowEntry{bitmap, startShadowUse}
	return bitmap
}
