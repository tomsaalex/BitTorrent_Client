package torrentclient

type peerMessageType = int

// Ordered so the const value matches up with the message ID
const (
	CHOKE_MESSAGE peerMessageType = iota
	UNCHOKE_MESSAGE
	INTERESTED_MESSAGE
	NOT_INTERESTED_MESSAGE
	HAVE_MESSAGE
	BITFIELD_MESSAGE
	REQUEST_MESSAGE
	PIECE_MESSAGE
	CANCEL_MESSAGE
	KEEP_ALIVE_MESSAGE
)

type peerMessage interface {
	Type() peerMessageType
}

type keepAliveMessage struct{}

func (kam keepAliveMessage) Type() peerMessageType {
	return KEEP_ALIVE_MESSAGE
}

type chokeMessage struct{}

func (cm chokeMessage) Type() peerMessageType {
	return CHOKE_MESSAGE
}

type unchokeMessage struct{}

func (um unchokeMessage) Type() peerMessageType {
	return UNCHOKE_MESSAGE
}

type interestedMessage struct{}

func (im interestedMessage) Type() peerMessageType {
	return INTERESTED_MESSAGE
}

type notInterestedMessage struct{}

func (nim notInterestedMessage) Type() peerMessageType {
	return NOT_INTERESTED_MESSAGE
}

type haveMessage struct {
	pieceIndex int
}

func (hm haveMessage) Type() peerMessageType {
	return HAVE_MESSAGE
}

type bitfieldMessage struct {
	bitfield []byte
}

func (bm bitfieldMessage) Type() peerMessageType {
	return BITFIELD_MESSAGE
}

type requestMessage struct {
	index  int
	begin  int
	length int
}

func (rm requestMessage) Type() peerMessageType {
	return REQUEST_MESSAGE
}

type pieceMessage struct {
	index int
	begin int
	block []byte
}

func (pm pieceMessage) Type() peerMessageType {
	return PIECE_MESSAGE
}

type cancelMessage struct {
	index  int
	begin  int
	length int
}

func (cm cancelMessage) Type() peerMessageType {
	return CANCEL_MESSAGE
}
