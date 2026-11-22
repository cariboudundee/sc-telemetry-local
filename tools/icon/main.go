package main

// Génère l'icône de l'application (cercle de visée cyan sur fond sombre).
import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	for _, s := range []int{256, 16} {
		img := image.NewNRGBA(image.Rect(0, 0, s, s))
		c := float64(s) / 2
		for y := 0; y < s; y++ {
			for x := 0; x < s; x++ {
				dx, dy := float64(x)+0.5-c, float64(y)+0.5-c
				r := math.Hypot(dx, dy) / c
				var col color.NRGBA
				switch {
				case r > 0.98:
					continue
				case r > 0.80:
					col = color.NRGBA{0x3e, 0xc7, 0xff, 255}
				case r < 0.22:
					col = color.NRGBA{0x3d, 0xdc, 0x84, 255}
				case math.Abs(dx) < c*0.06 || math.Abs(dy) < c*0.06:
					col = color.NRGBA{0x3e, 0xc7, 0xff, 200}
				default:
					col = color.NRGBA{0x0d, 0x14, 0x1c, 255}
				}
				img.SetNRGBA(x, y, col)
			}
		}
		name := "winres/icon.png"
		if s == 16 {
			name = "winres/icon16.png"
		}
		f, _ := os.Create(name)
		png.Encode(f, img)
		f.Close()
	}
}
