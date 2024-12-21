package torrentclient

import (
	"fmt"
	"strconv"
)

type PeerConnectionError struct {
	Message      string
	InvolvedPeer peer
}

func (pce *PeerConnectionError) Error() string {
	port := strconv.Itoa(int(pce.InvolvedPeer.port))
	return fmt.Sprintf("PeerConnectionError: %s. InvolvedPeer: IP: %s, Port: %s", pce.Message, pce.InvolvedPeer.ip, port)
}
