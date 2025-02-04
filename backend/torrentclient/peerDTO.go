package torrentclient

type peerDTO struct {
	peerID string `json:"peerID"`
	ip     string `json:"ip"`
	port   uint16 `json:"port"`
}
