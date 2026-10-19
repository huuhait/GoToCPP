package xz

import "errors"

// The LZMA decoder of the xz format's LZMA2 filter, after the LZMA specification (LzmaSpec.cpp of
// the LZMA SDK). It decodes one chunk at a time from a byte slice that holds the whole chunk
// (LZMA2 chunks are at most 64 KiB compressed and 2 MiB uncompressed), into a dictionary.

var errCorrupt = errors.New("xz: corrupt LZMA data")

const (
	numStates          = 12
	numPosBitsMax      = 4
	numLenToPosStates  = 4
	numAlignBits       = 4
	startPosModelIndex = 4
	endPosModelIndex   = 14
	numFullDistances   = 1 << (endPosModelIndex >> 1)
	matchMinLen        = 2
	probInit           = 1024 // half of 2048
	topValue           = 1 << 24
	numBitModelTotal   = 11
	numMoveBits        = 5
)

type prob uint16

// A rangeDecoder decodes bits from one chunk.
type rangeDecoder struct {
	in   []byte
	pos  int
	rng  uint32
	code uint32
	err  bool // read past the end of the chunk
}

func (d *rangeDecoder) next() byte {
	if d.pos >= len(d.in) {
		d.err = true
		return 0
	}
	b := d.in[d.pos]
	d.pos++
	return b
}

func (d *rangeDecoder) init(in []byte) error {
	d.in, d.pos, d.err = in, 0, false
	if len(in) < 5 || in[0] != 0 {
		return errCorrupt
	}
	d.pos = 1
	d.rng = 0xFFFFFFFF
	d.code = 0
	for i := 0; i < 4; i++ {
		d.code = d.code<<8 | uint32(d.next())
	}
	if d.code == d.rng {
		return errCorrupt
	}
	return nil
}

func (d *rangeDecoder) normalize() {
	if d.rng < topValue {
		d.rng <<= 8
		d.code = d.code<<8 | uint32(d.next())
	}
}

func (d *rangeDecoder) bit(p *prob) uint32 {
	v := uint32(*p)
	bound := (d.rng >> numBitModelTotal) * v
	var b uint32
	if d.code < bound {
		v += (1<<numBitModelTotal - v) >> numMoveBits
		d.rng = bound
	} else {
		v -= v >> numMoveBits
		d.code -= bound
		d.rng -= bound
		b = 1
	}
	*p = prob(v)
	d.normalize()
	return b
}

func (d *rangeDecoder) direct(n uint) uint32 {
	var res uint32
	for ; n > 0; n-- {
		d.rng >>= 1
		d.code -= d.rng
		t := 0 - (d.code >> 31)
		d.code += d.rng & t
		if d.code == d.rng {
			d.err = true // corrupt
		}
		d.normalize()
		res = res<<1 + t + 1
	}
	return res
}

// tree decodes n bits MSB first with the probabilities probs (probs[1:]).
func (d *rangeDecoder) tree(probs []prob, n uint) uint32 {
	m := uint32(1)
	for i := uint(0); i < n; i++ {
		m = m<<1 + d.bit(&probs[m])
	}
	return m - 1<<n
}

// reverseTree decodes n bits LSB first.
func (d *rangeDecoder) reverseTree(probs []prob, n uint) uint32 {
	m := uint32(1)
	var sym uint32
	for i := uint(0); i < n; i++ {
		b := d.bit(&probs[m])
		m = m<<1 + b
		sym |= b << i
	}
	return sym
}

type lenDecoder struct {
	choice, choice2 prob
	low             [1 << numPosBitsMax][1 << 3]prob
	mid             [1 << numPosBitsMax][1 << 3]prob
	high            [1 << 8]prob
}

func (l *lenDecoder) reset() {
	l.choice, l.choice2 = probInit, probInit
	for i := range l.low {
		for j := range l.low[i] {
			l.low[i][j], l.mid[i][j] = probInit, probInit
		}
	}
	for i := range l.high {
		l.high[i] = probInit
	}
}

func (l *lenDecoder) decode(d *rangeDecoder, posState uint32) uint32 {
	if d.bit(&l.choice) == 0 {
		return d.tree(l.low[posState][:], 3)
	}
	if d.bit(&l.choice2) == 0 {
		return 8 + d.tree(l.mid[posState][:], 3)
	}
	return 16 + d.tree(l.high[:], 8)
}

// A dictionary is the history that matches copy from, and the output of a chunk.
type dictionary struct {
	buf   []byte // the ring buffer
	pos   int    // the next position to write
	full  bool   // buf has been filled once
	total uint64 // bytes written since the last reset
	out   []byte // what was decoded since the reader last took it
}

func (d *dictionary) reset(size int) {
	if len(d.buf) != size {
		d.buf = make([]byte, size)
	}
	d.pos, d.full, d.total = 0, false, 0
}

func (d *dictionary) put(b byte) {
	d.buf[d.pos] = b
	d.pos++
	if d.pos == len(d.buf) {
		d.pos, d.full = 0, true
	}
	d.total++
	d.out = append(d.out, b)
}

// get returns the byte dist bytes back (1 is the last one written).
func (d *dictionary) get(dist uint32) byte {
	i := d.pos - int(dist)
	if i < 0 {
		i += len(d.buf)
	}
	return d.buf[i]
}

// has reports whether dist bytes of history exist.
func (d *dictionary) has(dist uint32) bool {
	return uint64(dist) <= d.total && int(dist) <= len(d.buf)
}

// An lzma is the decoder state that LZMA2 keeps across chunks.
type lzma struct {
	lc, lp, pb uint
	state      uint32
	rep        [4]uint32

	isMatch    [numStates << numPosBitsMax]prob
	isRep      [numStates]prob
	isRepG0    [numStates]prob
	isRepG1    [numStates]prob
	isRepG2    [numStates]prob
	isRep0Long [numStates << numPosBitsMax]prob
	posSlot    [numLenToPosStates][1 << 6]prob
	posSpecial [1 + numFullDistances - endPosModelIndex]prob
	align      [1 << numAlignBits]prob
	lit        []prob
	matchLen   lenDecoder
	repLen     lenDecoder

	rc rangeDecoder
}

// setProps sets lc, lp and pb from the properties byte and resets the state.
func (z *lzma) setProps(b byte) error {
	if b >= 9*5*5 {
		return errCorrupt
	}
	z.lc = uint(b % 9)
	b /= 9
	z.lp = uint(b % 5)
	z.pb = uint(b / 5)
	if z.lc+z.lp > 4 {
		return errCorrupt // LZMA2's limit
	}
	n := 0x300 << (z.lc + z.lp)
	if cap(z.lit) < n {
		z.lit = make([]prob, n)
	}
	z.lit = z.lit[:n]
	z.resetState()
	return nil
}

func (z *lzma) resetState() {
	z.state = 0
	z.rep = [4]uint32{}
	for i := range z.isMatch {
		z.isMatch[i], z.isRep0Long[i] = probInit, probInit
	}
	for i := 0; i < numStates; i++ {
		z.isRep[i], z.isRepG0[i], z.isRepG1[i], z.isRepG2[i] = probInit, probInit, probInit, probInit
	}
	for i := range z.posSlot {
		for j := range z.posSlot[i] {
			z.posSlot[i][j] = probInit
		}
	}
	for i := range z.posSpecial {
		z.posSpecial[i] = probInit
	}
	for i := range z.align {
		z.align[i] = probInit
	}
	for i := range z.lit {
		z.lit[i] = probInit
	}
	z.matchLen.reset()
	z.repLen.reset()
}

// decode decodes the chunk in (compressed) into dict until size bytes are written.
func (z *lzma) decode(dict *dictionary, in []byte, size int) error {
	if err := z.rc.init(in); err != nil {
		return err
	}
	rc := &z.rc
	pbMask := uint32(1)<<z.pb - 1
	lpMask := uint32(1)<<z.lp - 1
	written := 0
	for written < size {
		if rc.err {
			return errCorrupt
		}
		pos := uint32(dict.total)
		posState := pos & pbMask
		if rc.bit(&z.isMatch[z.state<<numPosBitsMax+posState]) == 0 {
			var prev uint32
			if dict.total > 0 {
				prev = uint32(dict.get(1))
			}
			litState := (pos&lpMask)<<z.lc + prev>>(8-z.lc)
			probs := z.lit[0x300*litState : 0x300*litState+0x300]
			sym := uint32(1)
			if z.state >= 7 {
				if !dict.has(z.rep[0] + 1) {
					return errCorrupt
				}
				matchByte := uint32(dict.get(z.rep[0] + 1))
				for sym < 0x100 {
					matchBit := (matchByte >> 7) & 1
					matchByte <<= 1
					b := rc.bit(&probs[(1+matchBit)<<8+sym])
					sym = sym<<1 | b
					if matchBit != b {
						break
					}
				}
			}
			for sym < 0x100 {
				sym = sym<<1 | rc.bit(&probs[sym])
			}
			dict.put(byte(sym))
			written++
			switch {
			case z.state < 4:
				z.state = 0
			case z.state < 10:
				z.state -= 3
			default:
				z.state -= 6
			}
			continue
		}
		var length uint32
		if rc.bit(&z.isRep[z.state]) == 0 {
			z.rep[3], z.rep[2], z.rep[1] = z.rep[2], z.rep[1], z.rep[0]
			length = z.matchLen.decode(rc, posState)
			if z.state < 7 {
				z.state = 7
			} else {
				z.state = 10
			}
			dist, err := z.distance(rc, length)
			if err != nil {
				return err
			}
			z.rep[0] = dist
		} else {
			if !dict.has(1) {
				return errCorrupt
			}
			if rc.bit(&z.isRepG0[z.state]) == 0 {
				if rc.bit(&z.isRep0Long[z.state<<numPosBitsMax+posState]) == 0 {
					// A short repeat: one byte from rep0.
					if z.state < 7 {
						z.state = 9
					} else {
						z.state = 11
					}
					if !dict.has(z.rep[0] + 1) {
						return errCorrupt
					}
					dict.put(dict.get(z.rep[0] + 1))
					written++
					continue
				}
			} else {
				var dist uint32
				if rc.bit(&z.isRepG1[z.state]) == 0 {
					dist = z.rep[1]
				} else {
					if rc.bit(&z.isRepG2[z.state]) == 0 {
						dist = z.rep[2]
					} else {
						dist = z.rep[3]
						z.rep[3] = z.rep[2]
					}
					z.rep[2] = z.rep[1]
				}
				z.rep[1] = z.rep[0]
				z.rep[0] = dist
			}
			length = z.repLen.decode(rc, posState)
			if z.state < 7 {
				z.state = 8
			} else {
				z.state = 11
			}
		}
		length += matchMinLen
		if !dict.has(z.rep[0]+1) || written+int(length) > size {
			return errCorrupt
		}
		for i := uint32(0); i < length; i++ {
			dict.put(dict.get(z.rep[0] + 1))
		}
		written += int(length)
	}
	if rc.err || rc.pos != len(in) || rc.code != 0 {
		return errCorrupt
	}
	return nil
}

// distance decodes the distance of a match whose length (minus matchMinLen) is length, and
// returns the zero-based distance (rep0).
func (z *lzma) distance(rc *rangeDecoder, length uint32) (uint32, error) {
	lenState := length
	if lenState > numLenToPosStates-1 {
		lenState = numLenToPosStates - 1
	}
	slot := rc.tree(z.posSlot[lenState][:], 6)
	if slot < startPosModelIndex {
		return slot, nil
	}
	direct := uint(slot>>1) - 1
	dist := (2 | slot&1) << direct
	if slot < endPosModelIndex {
		dist += rc.reverseTree(z.posSpecial[dist-slot:], direct)
		return dist, nil
	}
	dist += rc.direct(direct-numAlignBits) << numAlignBits
	dist += rc.reverseTree(z.align[:], numAlignBits)
	if dist == 0xFFFFFFFF {
		return 0, errCorrupt // the end marker, which LZMA2 does not use
	}
	return dist, nil
}
