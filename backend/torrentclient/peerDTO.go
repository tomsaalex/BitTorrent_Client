package torrentclient

type peerDTO struct {
	PeerID string `json:"peerID"`
	Ip     string `json:"ip"`
	Port   uint16 `json:"port"`
}
