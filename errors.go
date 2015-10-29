package p9pnew

import (
	"errors"
	"fmt"
)

// common errors returned by Session interface methods
var (
	ErrClosed = errors.New("closed")
)

// 9p wire errors returned by Session interface methods
var (
	ErrBadattach    = new9pError("unknown specifier in attach")
	ErrBadoffset    = new9pError("bad offset")
	ErrBadcount     = new9pError("bad count")
	ErrBotch        = new9pError("9P protocol botch")
	ErrCreatenondir = new9pError("create in non-directory")
	ErrDupfid       = new9pError("duplicate fid")
	ErrDuptag       = new9pError("duplicate tag")
	ErrIsdir        = new9pError("is a directory")
	ErrNocreate     = new9pError("create prohibited")
	ErrNomem        = new9pError("out of memory")
	ErrNoremove     = new9pError("remove prohibited")
	ErrNostat       = new9pError("stat prohibited")
	ErrNotfound     = new9pError("file not found")
	ErrNowrite      = new9pError("write prohibited")
	ErrNowstat      = new9pError("wstat prohibited")
	ErrPerm         = new9pError("permission denied")
	ErrUnknownfid   = new9pError("unknown fid")
	ErrBaddir       = new9pError("bad directory in wstat")
	ErrWalknodir    = new9pError("walk in non-directory")

	// extra errors not part of the normal protocol
	ErrTimeout = new9pError("fcall timeout") // returned when timing out on the fcall
)

type error9p struct {
	Name string
}

func new9pError(s string) error {
	return error9p{Name: s}
}

func (e error9p) Error() string {
	return fmt.Sprintf("9p: %v", e.Name)
}
