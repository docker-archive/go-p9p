package p9pnew

import "fmt"

type FcallType uint8

const (
	Tversion FcallType = iota + 100
	Rversion
	Tauth
	Rauth
	Tattach
	Rattach
	Terror
	Rerror
	Tflush
	Rflush
	Twalk
	Rwalk
	Topen
	Ropen
	Tcreate
	Rcreate
	Tread
	Rread
	Twrite
	Rwrite
	Tclunk
	Rclunk
	Tremove
	Rremove
	Tstat
	Rstat
	Twstat
	Rwstat
	Tmax
)

func (fct FcallType) String() string {
	switch fct {
	case Tversion:
		return "Tversion"
	case Rversion:
		return "Rversion"
	case Tauth:
		return "Tauth"
	case Rauth:
		return "Rauth"
	case Tattach:
		return "Tattach"
	case Rattach:
		return "Rattach"
	case Terror:
		// invalid.
		return "Terror"
	case Rerror:
		return "Rerror"
	case Tflush:
		return "Tflush"
	case Rflush:
		return "Rflush"
	case Twalk:
		return "Twalk"
	case Rwalk:
		return "Rwalk"
	case Topen:
		return "Topen"
	case Ropen:
		return "Ropen"
	case Tcreate:
		return "Tcreate"
	case Rcreate:
		return "Rcreate"
	case Tread:
		return "Tread"
	case Rread:
		return "Rread"
	case Twrite:
		return "Twrite"
	case Rwrite:
		return "Rwrite"
	case Tclunk:
		return "Tclunk"
	case Rclunk:
		return "Rclunk"
	case Tremove:
		return "Tremote"
	case Rremove:
		return "Rremove"
	case Tstat:
		return "Tstat"
	case Rstat:
		return "Rstat"
	case Twstat:
		return "Twstat"
	case Rwstat:
		return "Rwstat"
	default:
		return "Tunknown"
	}
}

type Fcall struct {
	Type    FcallType
	Tag     Tag
	Message Message
}

func newFcall(msg Message) *Fcall {
	return &Fcall{
		Type:    msg.Type(),
		Message: msg,
	}
}

func (fc *Fcall) String() string {
	return fmt.Sprintf("%8d %v(%v) %v", size9p(fc), fc.Type, fc.Tag, fc.Message)
}

type Message interface {
	// Type indicates the Fcall type of the message. This must match
	// Fcall.Type.
	Type() FcallType
}

// newMessage returns a new instance of the message based on the Fcall type.
func newMessage(typ FcallType) (Message, error) {
	// NOTE(stevvooe): This is a nasty bit of code but makes the transport
	// fairly simple to implement.
	switch typ {
	case Tversion:
		return &MessageTversion{}, nil
	case Rversion:
		return &MessageRversion{}, nil
	case Tauth:

	case Rauth:
	case Tattach:
		return &MessageTattach{}, nil
	case Rattach:
		return &MessageRattach{}, nil
	case Rerror:
		return &MessageRerror{}, nil
	case Tflush:
		return &MessageTflush{}, nil
	case Rflush:
		return nil, nil // No message body for this response.
	case Twalk:
		return &MessageTwalk{}, nil
	case Rwalk:
		return &MessageRwalk{}, nil
	case Topen:
		return &MessageTopen{}, nil
	case Ropen:
		return &MessageRopen{}, nil
	case Tcreate:

	case Rcreate:

	case Tread:
		return &MessageTread{}, nil
	case Rread:
		return &MessageRread{}, nil
	case Twrite:
		return &MessageTwrite{}, nil
	case Rwrite:
		return &MessageRwrite{}, nil
	case Tclunk:
		return &MessageTclunk{}, nil
	case Rclunk:
		return nil, nil // no response body
	case Tremove:

	case Rremove:

	case Tstat:

	case Rstat:
		return &MessageRstat{}, nil
	case Twstat:

	case Rwstat:

	}

	return nil, fmt.Errorf("unknown message type")
}

// MessageVersion encodes the message body for Tversion and Rversion RPC
// calls. The body is identical in both directions.
type MessageTversion struct {
	MSize   uint32
	Version string
}

type MessageRversion struct {
	MSize   uint32
	Version string
}

type MessageTauth struct {
	Afid  Fid
	Uname string
	Aname string
}

type MessageRauth struct {
	Qid Qid
}

type MessageRerror struct {
	Ename string
}

type MessageTflush struct {
	Oldtag Tag
}

type MessageTattach struct {
	Fid   Fid
	Afid  Fid
	Uname string
	Aname string
}

type MessageRattach struct {
	Qid Qid
}

type MessageTwalk struct {
	Fid    Fid
	Newfid Fid
	Wnames []string
}

type MessageRwalk struct {
	Qids []Qid
}

type MessageTopen struct {
	Fid  Fid
	Mode uint8
}

type MessageRopen struct {
	Qid   Qid
	Msize uint32
}

type MessageTcreate struct {
	Fid  Fid
	Name string
	Perm uint32
	Mode uint8
}

type MessageRcreate struct {
	Qid    Qid
	IOUnit uint32
}

type MessageTread struct {
	Fid    Fid
	Offset uint64
	Count  uint32
}

type MessageRread struct {
	Data []byte
}

type MessageTwrite struct {
	Fid    Fid
	Offset uint64
	Data   []byte
}

type MessageRwrite struct {
	Count uint32
}

type MessageTclunk struct {
	Fid Fid
}

type MessageTremove struct {
	Fid Fid
}

type MessageTstat struct {
	Fid Fid
}

type MessageRstat struct {
	Stat Dir
}

type MessageTwstat struct {
	Fid  Fid
	Stat Dir
}

func (MessageTversion) Type() FcallType { return Tversion }
func (MessageRversion) Type() FcallType { return Rversion }
func (MessageTauth) Type() FcallType    { return Tauth }
func (MessageRauth) Type() FcallType    { return Rauth }
func (MessageRerror) Type() FcallType   { return Rerror }
func (MessageTflush) Type() FcallType   { return Tflush }
func (MessageTattach) Type() FcallType  { return Tattach }
func (MessageRattach) Type() FcallType  { return Rattach }
func (MessageTwalk) Type() FcallType    { return Twalk }
func (MessageRwalk) Type() FcallType    { return Rwalk }
func (MessageTopen) Type() FcallType    { return Topen }
func (MessageRopen) Type() FcallType    { return Ropen }
func (MessageTcreate) Type() FcallType  { return Tcreate }
func (MessageRcreate) Type() FcallType  { return Rcreate }
func (MessageTread) Type() FcallType    { return Tread }
func (MessageRread) Type() FcallType    { return Rread }
func (MessageTwrite) Type() FcallType   { return Twrite }
func (MessageRwrite) Type() FcallType   { return Rwrite }
func (MessageTclunk) Type() FcallType   { return Tclunk }
func (MessageTremove) Type() FcallType  { return Tremove }
func (MessageTstat) Type() FcallType    { return Tstat }
func (MessageRstat) Type() FcallType    { return Rstat }
func (MessageTwstat) Type() FcallType   { return Twstat }
