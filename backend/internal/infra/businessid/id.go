// Package businessid generates the ULID business numbers required by BR rules.
package businessid

import (
	"crypto/rand"
	"math/big"
	"time"
)

func New(prefix string, now time.Time) (string, error) {
	var b [16]byte
	ms := uint64(now.UnixMilli())
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	if _, e := rand.Read(b[6:]); e != nil {
		return "", e
	}
	n := new(big.Int).SetBytes(b[:])
	alphabet := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	var chars [26]byte
	for i := 25; i >= 0; i-- {
		chars[i] = alphabet[new(big.Int).And(n, big.NewInt(31)).Int64()]
		n.Rsh(n, 5)
	}
	return prefix + string(chars[:]), nil
}
