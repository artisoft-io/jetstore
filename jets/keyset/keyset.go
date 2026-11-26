// Package keyset summarises a set of keys without keeping them: an
// order-independent fingerprint, which tells two sets equal or not, and a
// HyperLogLog sketch, which estimates how many keys two sets share.
//
// It is the one definition of how a key set is hashed. The claim-set checks
// compute it inside the database (observe.ClaimSetSummaryStatement) and a
// pipeline operator may compute it over its records; both must reach the same
// bits, so both follow the rules here, and this package's tests pin them. It
// imports the standard library only, so anything may import it.
//
// # The rules
//
// A key is one value per declared key column. Its text is, per column in the
// declared order, "1" followed by the value's text when the value is present,
// or "0" when it is null, the columns joined by U+001F (the unit separator).
// So a null and an empty string differ, and so do ("a", "") and ("", "a").
//
// A key's hash is the first 8 bytes of the SHA-256 of its text, as UTF-8,
// read big-endian.
//
// The fingerprint of a set is the sum of its distinct keys' hashes modulo
// 2^64. Two sets with equal fingerprints and equal distinct counts are equal
// but for a collision, whose chance is about one in 2^64. A sum, not a
// running hash, so the order in which keys arrive does not matter.
//
// The sketch is a HyperLogLog of 2^14 registers. A hash's top 14 bits name
// its register, and the register keeps the largest value seen of one more
// than the number of leading zeros among the hash's other 50 bits (51 when
// all 50 are zero). Packed, register i occupies bits 6i to 6i+5 of the byte
// string, most significant bit first: 12,288 bytes. Two sketches merge by
// taking each register's larger value, which is the sketch of the union.
//
// The estimate is the standard one (Flajolet et al., 2007): alpha_m m^2 over
// the sum of 2^-register, replaced by linear counting, m ln(m/V), when it is
// at most 2.5 m and V registers are still zero. A 64-bit hash needs no
// large-range correction.
//
// Two sets of exactly known sizes a and b share an estimated
// a + b - estimate(merged sketches) keys, and the error of that estimate is
// the error of the merged sketch's.
package keyset

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strings"
)

const (
	// Precision is the number of a hash's bits that name its register.
	Precision = 14
	// Registers is the number of registers in a sketch, 2^Precision.
	Registers = 1 << Precision
	// RegisterBits is the width of a packed register.
	RegisterBits = 6
	// SketchBytes is the size of a packed sketch: 12,288 bytes.
	SketchBytes = Registers * RegisterBits / 8
	// Separator joins the columns of a key's text.
	Separator = "\x1f"
	// maxRank is the largest register value: 50 zero bits, plus one.
	maxRank = 64 - Precision + 1
)

// KeyText is a key's text: per value, "1" and the value when it is present,
// or "0" when it is nil, joined by U+001F.
func KeyText(values []*string) string {
	var b strings.Builder
	for i, v := range values {
		if i > 0 {
			b.WriteString(Separator)
		}
		if v == nil {
			b.WriteByte('0')
			continue
		}
		b.WriteByte('1')
		b.WriteString(*v)
	}
	return b.String()
}

// Hash is a key text's hash: the first 8 bytes of its SHA-256, big-endian.
func Hash(text string) uint64 {
	sum := sha256.Sum256([]byte(text))
	return binary.BigEndian.Uint64(sum[:8])
}

// Sketch is a HyperLogLog of Registers registers, one byte each unpacked.
type Sketch [Registers]uint8

// Add records one hash.
func (s *Sketch) Add(h uint64) {
	i := h >> (64 - Precision)
	rank := uint8(bits.LeadingZeros64(h<<Precision)) + 1
	if rank > maxRank {
		rank = maxRank
	}
	if rank > s[i] {
		s[i] = rank
	}
}

// Merge makes s the sketch of the union of s and o.
func (s *Sketch) Merge(o *Sketch) {
	for i, v := range o {
		if v > s[i] {
			s[i] = v
		}
	}
}

// Merged is the sketch of the union of a and b; neither is changed.
func Merged(a, b *Sketch) *Sketch {
	out := *a
	out.Merge(b)
	return &out
}

// Estimate is the sketch's estimate of its distinct count, with the
// standard small-range correction.
func (s *Sketch) Estimate() float64 {
	est, _ := s.EstimateWithError()
	return est
}

// EstimateWithError is the estimate and one standard error of it. Where the
// estimate is linear counting, the standard error is sqrt(m (e^t - t - 1)),
// t = estimate / m (Whang et al., 1990); elsewhere it is 1.04/sqrt(m) of the
// estimate, about 0.81% at 2^14 registers. Both are the error of a random set
// of the estimated size, not a bound on any one set.
func (s *Sketch) EstimateWithError() (estimate, stderr float64) {
	const m = float64(Registers)
	sum := 0.0
	zeros := 0
	for _, v := range s {
		sum += math.Ldexp(1, -int(v))
		if v == 0 {
			zeros++
		}
	}
	alpha := 0.7213 / (1 + 1.079/m)
	raw := alpha * m * m / sum
	if raw <= 2.5*m && zeros > 0 {
		est := m * math.Log(m/float64(zeros))
		t := est / m
		return est, math.Sqrt(m * (math.Exp(t) - t - 1))
	}
	return raw, RelativeError * raw
}

// RelativeError is the standard error of an estimate outside the
// linear-counting range, as a share of it: 1.04/sqrt(2^14).
var RelativeError = 1.04 / math.Sqrt(Registers)

// Pack writes the sketch as 6 bits a register, most significant bit first.
func (s *Sketch) Pack() []byte {
	out := make([]byte, SketchBytes)
	for g := 0; g < Registers/4; g++ {
		w := uint32(s[4*g])<<18 | uint32(s[4*g+1])<<12 | uint32(s[4*g+2])<<6 | uint32(s[4*g+3])
		out[3*g], out[3*g+1], out[3*g+2] = byte(w>>16), byte(w>>8), byte(w)
	}
	return out
}

// Unpack reads a packed sketch.
func Unpack(b []byte) (*Sketch, error) {
	if len(b) != SketchBytes {
		return nil, fmt.Errorf("a packed sketch is %d bytes, not %d", SketchBytes, len(b))
	}
	var s Sketch
	for g := 0; g < Registers/4; g++ {
		w := uint32(b[3*g])<<16 | uint32(b[3*g+1])<<8 | uint32(b[3*g+2])
		for j := 0; j < 4; j++ {
			v := uint8(w>>(18-6*j)) & 0x3f
			if v > maxRank {
				return nil, errors.New("a packed sketch holds a register above 51, which no hash produces")
			}
			s[4*g+j] = v
		}
	}
	return &s, nil
}

// Shared is the estimated number of keys two sets of exactly a and b distinct
// keys share, from their sketches, and one standard error of it, which is the
// merged sketch's. The estimate is held between 0 and the smaller set.
func Shared(a, b int64, sa, sb *Sketch) (shared, stderr float64) {
	union, se := Merged(sa, sb).EstimateWithError()
	shared = float64(a+b) - union
	shared = math.Max(0, math.Min(shared, float64(min(a, b))))
	return shared, se
}

// Bound is the error stated with an estimate of shared keys: three standard
// errors, plus three keys. In the linear-counting range the error is the
// count of register collisions, close to a Poisson count of mean n^2/2m,
// whose tail at a mean of a few keys is heavier than three standard errors
// allow: 0 shared keys between deliveries of about 150 keys each was once
// estimated at 6 where three standard errors were 4.9. Three keys more covers
// that tail, and is negligible beside a large set's error.
func Bound(stderr float64) float64 { return 3*stderr + 3 }

// Summary is a set's fingerprint, sketch and distinct count, built one key at
// a time. A key added twice counts once: Builder remembers the hashes it has
// seen, so its memory is 8 bytes and a map entry per distinct key.
type Summary struct {
	Count       int64
	Fingerprint uint64
	Sketch      Sketch
}

// Builder builds a Summary from keys that may repeat.
type Builder struct {
	s    Summary
	seen map[uint64]struct{}
}

// NewBuilder is an empty builder.
func NewBuilder() *Builder { return &Builder{seen: map[uint64]struct{}{}} }

// AddText adds one key by its text (KeyText).
func (b *Builder) AddText(text string) { b.AddHash(Hash(text)) }

// AddHash adds one key by its hash.
func (b *Builder) AddHash(h uint64) {
	if _, ok := b.seen[h]; ok {
		return
	}
	b.seen[h] = struct{}{}
	b.s.Count++
	b.s.Fingerprint += h
	b.s.Sketch.Add(h)
}

// Summary is what has been added so far.
func (b *Builder) Summary() Summary { return b.s }
