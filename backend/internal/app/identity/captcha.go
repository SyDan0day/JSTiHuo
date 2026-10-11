package identity

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/big"
)

var segments = []string{"abcdef", "bc", "abdeg", "abcdg", "bcfg", "acdfg", "acdefg", "abc", "abcdefg", "abcdfg"}

func captcha() (string, string, error) {
	img := image.NewRGBA(image.Rect(0, 0, 150, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 150; x++ {
			img.Set(x, y, color.RGBA{245, 245, 245, 255})
		}
	}
	answer := ""
	coords := map[rune][4]int{'a': {3, 2, 20, 5}, 'b': {20, 5, 23, 20}, 'c': {20, 23, 23, 38}, 'd': {3, 38, 20, 41}, 'e': {0, 23, 3, 38}, 'f': {0, 5, 3, 20}, 'g': {3, 20, 20, 23}}
	for i := 0; i < 4; i++ {
		v, e := rand.Int(rand.Reader, big.NewInt(10))
		if e != nil {
			return "", "", e
		}
		d := int(v.Int64())
		answer += string(rune('0' + d))
		for _, segment := range segments[d] {
			r := coords[segment]
			for y := r[1]; y < r[3]; y++ {
				for x := r[0]; x < r[2]; x++ {
					img.Set(12+i*34+x, 8+y, color.RGBA{60, 45, 80, 255})
				}
			}
		}
	}
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		return "", "", e
	}
	return answer, "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()), nil
}
