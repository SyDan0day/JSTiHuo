package files

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

type Image struct {
	Data []byte
	SHA  []byte
	Key  string
}

func PrepareImage(raw []byte) (Image, error) {
	if len(raw) == 0 || len(raw) > 5*1024*1024 {
		return Image{}, fmt.Errorf("image size invalid")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || (format != "png" && format != "jpeg") || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16000000 {
		return Image{}, fmt.Errorf("image format or dimensions invalid")
	}
	img, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return Image{}, fmt.Errorf("image decode failed")
	}
	var out bytes.Buffer
	if e = png.Encode(&out, img); e != nil {
		return Image{}, e
	}
	if out.Len() > 10000000 {
		return Image{}, fmt.Errorf("encoded image too large")
	}
	name := make([]byte, 16)
	if _, e = rand.Read(name); e != nil {
		return Image{}, e
	}
	hash := sha256.Sum256(out.Bytes())
	return Image{out.Bytes(), hash[:], hex.EncodeToString(name) + ".png"}, nil
}
func WriteImage(root string, img Image) error {
	f, e := os.CreateTemp(root, ".staged-")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(img.Data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp, filepath.Join(root, img.Key))
}
func OpenImage(root, key string) (*os.File, error) {
	if len(key) != 36 || filepath.Base(key) != key || filepath.Ext(key) != ".png" {
		return nil, os.ErrNotExist
	}
	return os.Open(filepath.Join(root, key))
}
