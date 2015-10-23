package p9pnew

import (
	"errors"
	"fmt"
)

type Error struct {
	Name string
}

func (e Error) Error() string {
	return fmt.Sprintf("9p: %v", e.Name)
}

var (
	ErrClosed = errors.New("closed")

	ErrUnknownfid = Error{Name: "unknown fid"}
)
