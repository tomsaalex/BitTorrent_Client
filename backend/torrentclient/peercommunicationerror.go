package torrentclient

import (
	"fmt"
)

type PeerCommunicationError struct {
	Message      string
	InvolvedPeer peer
}

func (pce *PeerCommunicationError) Error() string {
	return fmt.Sprintf("PeerCommunicationError: %s. InvolvedPeer: IP: %s, Port: %s", pce.Message, pce.InvolvedPeer.ip, pce.InvolvedPeer.port)
}
