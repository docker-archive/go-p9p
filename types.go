package p9pnew

import "time"

const (
	NOFID = ^Fid(0)
	NOTAG = ^Tag(0)
)

const (
	DMDIR    = 0x80000000 // mode bit for directories
	DMAPPEND = 0x40000000 // mode bit for append only files
	DMEXCL   = 0x20000000 // mode bit for exclusive use files
	DMMOUNT  = 0x10000000 // mode bit for mounted channel
	DMAUTH   = 0x08000000 // mode bit for authentication file
	DMTMP    = 0x04000000 // mode bit for non-backed-up files
	DMREAD   = 0x4        // mode bit for read permission
	DMWRITE  = 0x2        // mode bit for write permission
	DMEXEC   = 0x1        // mode bit for execute permission
)

const (
	OREAD   = 0      // open for read
	OWRITE  = 1      // write
	ORDWR   = 2      // read and write
	OEXEC   = 3      // execute, == read but check execute permission
	OTRUNC  = 16     // or'ed in (except for exec), truncate file first
	OCEXEC  = 32     // or'ed in, close on exec
	ORCLOSE = 64     // or'ed in, remove on close
	OEXCL   = 0x1000 // or'ed in, exclusive use (create only)
)

type QType uint8

const (
	QTDIR    QType = 0x80 // type bit for directories
	QTAPPEND QType = 0x40 // type bit for append only files
	QTEXCL   QType = 0x20 // type bit for exclusive use files
	QTMOUNT  QType = 0x10 // type bit for mounted channel
	QTAUTH   QType = 0x08 // type bit for authentication file
	QTTMP    QType = 0x04 // type bit for not-backed-up file
	QTFILE   QType = 0x00 // plain file
)

type Fid uint32

type Qid struct {
	Type    QType
	Version uint32
	Path    uint64
}

type Dir struct {
	Type       uint16
	Dev        uint32
	Qid        Qid
	Mode       uint32
	AccessTime time.Time
	ModTime    time.Time
	Length     uint64
	Name       string
	UID        string
	GID        string
	MUID       string

	// TODO(stevvooe): 9p2000.u/L should go here.
}

//
type Tag uint16
