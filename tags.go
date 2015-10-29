package p9pnew

import "fmt"

// tagPool implements a free list to manage tags for outstanding 9p requests.
type tagPool struct {
	maximum  Tag
	freelist chan Tag // buffered to maximum
	nexttag  chan Tag // synchronous until max allocated
	closed   chan struct{}
}

func (tp *tagPool) Get() Tag {
	select {
	case <-tp.closed:
		panic("tag pool is closed")
	case t := <-tp.freelist:
		return t
	case t := <-tp.nexttag:
		return t
	}
}

func (tp *tagPool) next() {
	var next Tag

	for {
		select {
		case <-tp.closed:
			return
		case tp.nexttag <- next:
			next++

			if next >= tp.maximum {
				return // exhausted, exit this loop
			}
		}
	}
}

func (tp *tagPool) Put(tag Tag) {
	select {
	case tp.freelist <- tag:
	case <-tp.closed:
	}
}

func (tp *tagPool) Close() error {
	select {
	case <-tp.closed:
		return fmt.Errorf("closed")
	default:
		close(tp.closed)
	}
	return nil
}

// NewtagPool returns a tag pool with the maximum number of outstanding
// requests.
func newTagPool(outstanding int) *tagPool {
	return &tagPool{
		maximum:  Tag(outstanding),
		freelist: make(chan Tag, outstanding),
		nexttag:  make(chan Tag),
		closed:   make(chan struct{}),
	}
}
