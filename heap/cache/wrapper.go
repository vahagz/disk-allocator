package cache

import (
	"encoding"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	allocator "github.com/vahagz/disk-allocator/heap"
)

type LOCKMODE int

const (
	NONE LOCKMODE = iota
	READ
	WRITE
)

type binaryMarshalerUnmarshaler interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type Dirtyable interface {
	IsDirty() bool
	Dirty(v bool)
}

type pointable interface {
	binaryMarshalerUnmarshaler
	Dirtyable
	IsNil() bool
}

type Pointable[T pointable] interface {
	RLock() *pointerWrapper[T]
	RUnlock() *pointerWrapper[T]
	Lock() *pointerWrapper[T]
	Unlock() *pointerWrapper[T]
	LockFlag(flag LOCKMODE) *pointerWrapper[T]
	UnlockFlag(flag LOCKMODE) *pointerWrapper[T]
	New() T
	Get() T
	Set(val T) *pointerWrapper[T]
	Flush() *pointerWrapper[T]
	Ptr() allocator.Pointable

	binaryMarshalerUnmarshaler
}

type pointerWrapper[T pointable] struct {
	cache    *Cache[T]
	ptr      allocator.Pointable
	val      T
	accessed bool
	flush    bool
	mutex    *sync.RWMutex
}

func (p *pointerWrapper[T]) lock() *pointerWrapper[T] {
	p.cache.locked[p.ptr.Addr()] = p
	if item, ok := p.cache.rlocked[p.ptr.Addr()]; ok {
		item.readers++
	} else {
		p.cache.rlocked[p.ptr.Addr()] = &rlock[T]{ ptr: p, readers: 1 }
	}
	return p
}

func (p *pointerWrapper[T]) unlock() *pointerWrapper[T] {
	delete(p.cache.locked, p.ptr.Addr())
	if item, ok := p.cache.rlocked[p.ptr.Addr()]; ok && item.readers > 1 {
		item.readers--
	} else {
		delete(p.cache.rlocked, p.ptr.Addr())
	}

	if p.flush {
		p.Flush()
		p.flush = false
	}
	return p
}

func (p *pointerWrapper[T]) RLock() *pointerWrapper[T] {
	p.cache.mutex.Lock()
	p.lock()
	p.cache.mutex.Unlock()
	p.mutex.RLock()
	return p
}

func (p *pointerWrapper[T]) RUnlock() *pointerWrapper[T] {
	p.cache.mutex.Lock()
	p.unlock()
	p.cache.mutex.Unlock()
	p.mutex.RUnlock()
	return p
}

func (p *pointerWrapper[T]) Lock() *pointerWrapper[T] {
	p.cache.mutex.Lock()
	p.lock()
	p.cache.mutex.Unlock()
	p.mutex.Lock()
	return p
}

func (p *pointerWrapper[T]) Unlock() *pointerWrapper[T] {
	p.cache.mutex.Lock()
	p.unlock()
	p.cache.mutex.Unlock()
	p.mutex.Unlock()
	return p
}

func (p *pointerWrapper[T]) LockFlag(flag LOCKMODE) *pointerWrapper[T] {
	switch flag {
		case READ:
			p.RLock()
			break
		case WRITE:
			p.Lock()
			break
	}
	return p
}

func (p *pointerWrapper[T]) UnlockFlag(flag LOCKMODE) *pointerWrapper[T] {
	switch flag {
		case READ:
			p.RUnlock()
			break
		case WRITE:
			p.Unlock()
			break
	}
	return p
}

func (p *pointerWrapper[T]) New() T {
	p.accessed = true
	p.val = p.cache.newItem()
	return p.val
}

func (p *pointerWrapper[T]) Get() T {
	if p.accessed {
		return p.val
	}

	itm := p.New()
	if err := p.ptr.Get(itm); err != nil {
		panic(errors.Wrap(err, allocator.ErrUnmarshal.Error()))
	}

	return itm
}

func (p *pointerWrapper[T]) Set(val T) *pointerWrapper[T] {
	p.accessed = true
	p.val = val
	val.Dirty(true)
	return p
}

func (p *pointerWrapper[T]) Flush() *pointerWrapper[T] {
	_, locked := p.cache.locked[p.ptr.Addr()]
	_, rlocked := p.cache.rlocked[p.ptr.Addr()]
	if p.val.IsNil() || !p.val.IsDirty() || locked || rlocked {
		if locked || rlocked {
			p.flush = true
		}
		return p
	}

	if err := p.ptr.Set(p.val); err != nil {
		panic(errors.Wrap(err, allocator.ErrMarshal.Error()))
	}
	p.val.Dirty(false)
	return p
}

func (p *pointerWrapper[T]) Ptr() allocator.Pointable {
	return p.ptr
}

func (p *pointerWrapper[T]) MarshalBinary() ([]byte, error) {
	return p.ptr.MarshalBinary()
}

func (p *pointerWrapper[T]) UnmarshalBinary(d []byte) error {
	return p.ptr.UnmarshalBinary(d)
}

func (p *pointerWrapper[T]) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("%v -> %v", p.ptr, p.val)))
}
