package torrentclient

import (
	"strconv"

	"github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"
)

type peer struct {
	peerID customdatatypes.CustomHash // Optional. Doesn't appear if response comes in compact form
	ip     string
	port   uint16
}

func (p *peer) equal(other *peer) bool {
	return p.ip == other.ip && p.port == other.port
}

func (p *peer) fullAddress() string {
	return p.ip + ":" + strconv.FormatUint(uint64(p.port), 10)
}
