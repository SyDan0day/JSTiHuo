package crypto

import (
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	a, e := Hash("Correct-password-123")
	if e != nil {
		t.Fatal(e)
	}
	b, e := Hash("Correct-password-123")
	if e != nil {
		t.Fatal(e)
	}
	if a == b {
		t.Fatal("salt reused")
	}
	if !Verify("Correct-password-123", a) || Verify("incorrect-password", a) || Verify("Correct-password-123 ", a) {
		t.Fatal("password verification mismatch")
	}
	if Verify("Correct-password-123", strings.Replace(a, "m=65536", "m=999999999", 1)) {
		t.Fatal("unbounded hash accepted")
	}
}
