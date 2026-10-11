package catalog

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	for _, in := range []string{`<p onclick="alert(1)">hello <b>gift</b></p><script>alert(2)</script>`, `<svg><script>alert(1)</script></svg><a href="javascript:alert(1)">link</a>`, `<img src=x onerror=alert(1)><iframe srcdoc="bad">bad</iframe>`, `<p><strong><em>text & more</em></strong>`} {
		out, e := Sanitize(in)
		if e != nil {
			t.Fatal(e)
		}
		for _, bad := range []string{"onclick", "script", "svg", "javascript:", "onerror", "iframe", "href=", "<img"} {
			if strings.Contains(out, bad) {
				t.Fatalf("unsafe output %q", out)
			}
		}
		again, e := Sanitize(out)
		if e != nil || again != out {
			t.Fatal("sanitization is not stable")
		}
	}
}
