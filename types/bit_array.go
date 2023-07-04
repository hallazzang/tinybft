package types

type BitArray struct {
	bits  int
	elems []uint64
}

func NewBitArray(bits int) *BitArray {
	return &BitArray{
		bits:  bits,
		elems: make([]uint64, (bits+63)/64),
	}
}

func (ba *BitArray) GetIndex(i int) bool {
	if i >= ba.bits {
		panic("i >= bits")
	}
	return ba.elems[i/64]&(uint64(1)<<(i%64)) > 0
}

func (ba *BitArray) SetIndex(i int, v bool) {
	if i >= ba.bits {
		panic("i >= bits")
	}
	if v {
		ba.elems[i/64] |= uint64(1) << (i % 64)
	} else {
		ba.elems[i/64] &= ^(uint64(1) << (i % 64))
	}
}
