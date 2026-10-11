package files

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"testing"
)

func TestImages(t *testing.T) {
	var raw bytes.Buffer
	png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	img, e := PrepareImage(raw.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	if e = WriteImage(dir, img); e != nil {
		t.Fatal(e)
	}
	f, e := OpenImage(dir, img.Key)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	stored, _ := io.ReadAll(f)
	if !bytes.Equal(img.Data, stored) {
		t.Fatal("image changed")
	}
	huge := append([]byte(nil), raw.Bytes()...)
	binary.BigEndian.PutUint32(huge[16:20], 10000)
	binary.BigEndian.PutUint32(huge[20:24], 10000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, e = PrepareImage(huge); e == nil {
		t.Fatal("pixel limit bypassed")
	}
	for _, b := range [][]byte{[]byte(`<svg onload="alert(1)"></svg>`), []byte(`<html>test</html>`), raw.Bytes()[:20], make([]byte, 5*1024*1024+1)} {
		if _, e = PrepareImage(b); e == nil {
			t.Fatal("invalid image accepted")
		}
	}
	if _, e = OpenImage(dir, "../secret.png"); e == nil {
		t.Fatal("path traversal accepted")
	}
}
