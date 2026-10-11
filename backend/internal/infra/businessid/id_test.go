package businessid

import (
	"encoding/binary"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestNumber(t *testing.T) {
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, e := New("I_", now)
		if e != nil {
			t.Fatal(e)
		}
		if len(id) != 28 || !strings.HasPrefix(id, "I_") || seen[id] {
			t.Fatal("invalid or duplicate number")
		}
		seen[id] = true
		n := big.NewInt(0)
		for _, c := range id[2:] {
			digit := strings.IndexRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c)
			if digit < 0 {
				t.Fatal("invalid alphabet")
			}
			n.Lsh(n, 5)
			n.Or(n, big.NewInt(int64(digit)))
		}
		b := make([]byte, 16)
		n.FillBytes(b)
		var timestamp [8]byte
		copy(timestamp[2:], b[:6])
		if int64(binary.BigEndian.Uint64(timestamp[:])) != now.UnixMilli() {
			t.Fatal("timestamp not preserved")
		}
	}
}
