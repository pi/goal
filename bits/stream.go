package bits

import "github.com/pi/goal/md"

var _ = md.BitsPerUint

type BitStream struct {
	s   *BitSlice
	pos uint
}

func NewBitStream() *BitStream {
	return &BitStream{}
}

func NewBitStreamOn(bits []byte) *BitStream {
	s := &BitStream{}
	s.s = NewBitSlice()
	s.s.AppendBytes(bits)
	return s
}

func (s *BitStream) Bytes() []byte {
	if s.s == nil {
		return nil
	} else {
		return s.s.Bytes()
	}
}

func (s *BitStream) Pos() uint {
	return s.pos
}

func (s *BitStream) Rewind() {
	s.pos = 0
}

func (s *BitStream) Trunc() {
	if s.s != nil {
		s.s.SetLen(s.pos)
	}
}

func (s *BitStream) SetLen(newLen uint) {
	sp := s.pos
	s.SetPos(newLen)
	if s.pos < s.s.len {
		s.Trunc()
	}
	if sp < s.pos {
		s.pos = sp
	}
}

func (s *BitStream) SetPos(newPos uint) {
	if s.s == nil {
		s.s = NewBitSlice()
	}
	if newPos > s.s.len {
		s.s.SetLen(newPos)
	}
	s.pos = newPos
}

func (s *BitStream) Skip(n uint) {
	s.SetPos(s.pos + n)
}

func (s *BitStream) Len() uint {
	if s.s == nil {
		return 0
	} else {
		return s.s.len
	}
}

func (s *BitStream) Read(n uint) (uint, uint) {
	if s.pos+n > s.s.len {
		n = s.s.len - s.pos
	}
	v := s.s.GetBitRange(s.pos, s.pos+n-1)
	s.pos += n
	return v, n
}

func (s *BitStream) Write(n, bits uint) {
	if s.s == nil {
		s.s = NewBitSlice()
	}
	if s.pos == s.s.len {
		s.s.AppendBits(n, bits)
		s.pos += n
	} else {
		if s.pos+n > s.s.len {
			s.s.AppendBits(s.pos+n-s.s.len, 0)
		}
		s.s.PutBitRange(s.pos, s.pos+n-1, bits)
		s.pos += n
	}
}
