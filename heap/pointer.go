package allocator

import (
	"encoding"
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vahagz/pager"
)

var bin = binary.BigEndian
var ErrInvalidPointer = errors.New("invalid Pointer")
var ErrUnmarshal = errors.New("unmarshal error")
var ErrMarshal = errors.New("marshal error")

const PointerSize = 8 + PointerMetaSize

type binaryMarshalerUnmarshaler interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type Pointable interface {
	Get(into encoding.BinaryUnmarshaler) error
	Set(from encoding.BinaryMarshaler) error
	Addr() uint64
	Size() uint32
	IsFree() bool
	IsNil() bool
	Copy() Pointable
	Equal(ptr Pointable) bool
	binaryMarshalerUnmarshaler
}

type Pointer struct {
	ptr   uint64
	meta  *pointerMetadata
	pager *pager.Pager
}

func (p *Pointer) Get(into encoding.BinaryUnmarshaler) error {
	buf := make([]byte, p.meta.size)
	if err := p.pager.ReadAt(buf, p.ptr); err != nil {
		return ErrInvalidPointer
	}
	if err := into.UnmarshalBinary(buf); err != nil {
		return ErrUnmarshal
	}
	return nil
}

func (p *Pointer) Set(from encoding.BinaryMarshaler) error {
	bytes, err := from.MarshalBinary()
	if err != nil {
		return ErrMarshal
	}
	if err := p.pager.WriteAt(bytes, p.ptr); err != nil {
		return ErrInvalidPointer
	}
	return nil
}

func (p *Pointer) Addr() uint64 {
	return p.ptr
}

func (p *Pointer) Size() uint32 {
	return p.meta.size
}

func (p *Pointer) IsFree() bool {
	return p.meta.free
}

func (p *Pointer) IsNil() bool {
	return p.ptr == 0
}

func (p *Pointer) Copy() Pointable {
	return &Pointer{p.ptr,&pointerMetadata{},p.pager}
}

func (p *Pointer) Equal(ptr Pointable) bool {
	return p.Addr() == ptr.Addr() && p.Size() == ptr.Size() && p.IsFree() == ptr.IsFree()
}

func (p *Pointer) MarshalBinary() ([]byte, error) {
	buf := make([]byte, PointerSize)

	if metaBytes, err := p.meta.MarshalBinary(); err != nil {
		return nil, err
	} else {
		copy(buf[0:PointerMetaSize], metaBytes)
	}
	bin.PutUint64(buf[PointerMetaSize:PointerMetaSize+8], p.ptr)

	return buf, nil
}

func (p *Pointer) UnmarshalBinary(d []byte) error {
	if err := p.meta.UnmarshalBinary(d[0:PointerMetaSize]); err != nil {
		return err
	}
	p.ptr = bin.Uint64(d[PointerMetaSize:PointerMetaSize+8])
	return nil
}

func (p *Pointer) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("{ptr:'%v', size:'%v', free:'%v'}", p.ptr, p.meta.size, p.meta.free)))
}

func (p *Pointer) key() *freelistKey {
	return &freelistKey{
		ptr:  p.ptr - PointerMetaSize,
		size: p.meta.size + 2 * PointerMetaSize,
	}
}

func (p *Pointer) next() (*Pointer, error) {
	nextPtrMeta := &pointerMetadata{}
	nextPtrMetaBytes := make([]byte, PointerMetaSize)
	err := p.pager.ReadAt(nextPtrMetaBytes, p.ptr + uint64(p.meta.size) + PointerMetaSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read next Pointer meta")
	}

	err = nextPtrMeta.UnmarshalBinary(nextPtrMetaBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal next Pointer meta")
	}

	return &Pointer{
		ptr:   p.ptr + uint64(p.meta.size) + 2 * PointerMetaSize,
		meta:  nextPtrMeta,
		pager: p.pager,
	}, nil
}

func (p *Pointer) prev() (*Pointer, error) {
	prevPtrMeta := &pointerMetadata{}
	prevPtrMetaBytes := make([]byte, PointerMetaSize)
	err := p.pager.ReadAt(prevPtrMetaBytes, p.ptr - 2 * PointerMetaSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read prev Pointer meta")
	}

	err = prevPtrMeta.UnmarshalBinary(prevPtrMetaBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal prev Pointer meta")
	}

	return &Pointer{
		ptr:   p.ptr - uint64(prevPtrMeta.size) - 2 * PointerMetaSize,
		meta:  prevPtrMeta,
		pager: p.pager,
	}, nil
}

func (p *Pointer) writeMeta() error {
	bytes, err := p.meta.MarshalBinary()
	if err != nil {
		return ErrMarshal
	}
	if err := p.pager.WriteAt(bytes, p.ptr - PointerMetaSize); err != nil {
		return ErrInvalidPointer
	}
	if err := p.pager.WriteAt(bytes, p.ptr + uint64(p.meta.size)); err != nil {
		return ErrInvalidPointer
	}
	return nil
}
