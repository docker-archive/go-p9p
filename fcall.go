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

func (fc Fcall) String() string {
	return fmt.Sprintf("%8d %v(%v) %v", size9p(fc), fc.Type, fc.Tag, fc.Message)
}

type Message interface {
	// Size() uint32

	// NOTE(stevvooe): The binary marshal approach isn't particularly nice to
	// generating garbage. Consider using an append model, once we have the
	// messages worked out.
	// encoding.BinaryMarshaler
	// encoding.BinaryUnmarshaler

	message9p()
}

// newMessage returns a new instance of the message based on the Fcall type.
func newMessage(typ FcallType) (Message, error) {
	// NOTE(stevvooe): This is a nasty bit of code but makes the transport
	// fairly simple to implement.
	switch typ {
	case Tversion, Rversion:
		return &MessageVersion{}, nil
	case Tauth:

	case Rauth:

	case Tattach:

	case Rattach:

	case Terror:

	case Rerror:

	case Tflush:
		return &MessageTflush{}, nil
	case Rflush:
		return nil, nil // No message body for this response.
	case Twalk:
		return &MessageTwalk{}, nil
	case Rwalk:
		return &MessageRwalk{}, nil
	case Topen:

	case Ropen:

	case Tcreate:

	case Rcreate:

	case Tread:

	case Rread:
		return &MessageRread{}, nil
	case Twrite:

	case Rwrite:

	case Tclunk:

	case Rclunk:

	case Tremove:

	case Rremove:

	case Tstat:

	case Rstat:
		return &MessageRstat{}, nil
	case Twstat:

	case Rwstat:
	default:
		return nil, fmt.Errorf("unknown message type: %v", typ)

	}

	return nil, fmt.Errorf("unknown message")
}

// MessageVersion encodes the message body for Tversion and Rversion RPC
// calls. The body is identical in both directions.
type MessageVersion struct {
	MSize   uint32
	Version string
}

func (MessageVersion) message9p() {}

func (mv MessageVersion) String() string {
	return fmt.Sprintf("msize=%v version=%v", mv.MSize, mv.Version)
}

type MessageTAuth struct {
	Afid  Fid
	Uname string
	Aname string
}

type MessageRAuth struct {
	Qid Qid
}

type MessageRerror struct {
	Ename string
}

// MessageTflush handles the content for the Tflush message type.
type MessageTflush struct {
	Oldtag Tag
}

func (MessageTflush) message9p() {}

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
	Wname  []string
}

func (MessageTwalk) message9p() {}

type MessageRwalk struct {
	Qids []Qid
}

func (MessageRwalk) message9p() {}

type MessageTopen struct {
	Fid  Fid
	Mode uint8
}

func (MessageTopen) message9p() {}

type MessageRopen struct {
	Qid   Qid
	Msize uint32
}

func (MessageRopen) message9p() {}

type MessageTcreate struct {
	Fid  Fid
	Name string
	Perm uint32
	Mode uint8
}

func (MessageTcreate) message9p() {}

type MessageRcreate struct {
	Qid    Qid
	IOUnit uint32
}

func (MessageRcreate) message9p() {}

type MessageTread struct {
	Fid    Fid
	Offset uint64
	Count  uint32
}

func (MessageTread) message9p() {}

type MessageRread struct {
	Data []byte
}

func (MessageRread) message9p() {}

type MessageTwrite struct {
	Fid    Fid
	Offset uint64
	Data   []byte
}

func (MessageTwrite) message9p() {}

type MessageRwrite struct {
	Count uint32
}

func (MessageRwrite) message9p() {}

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

func (MessageRstat) message9p() {}

type MessageTwstat struct {
	Fid  Fid
	Stat Dir
}
