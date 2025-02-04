package torrentclient

type TorrentDTO struct {
	TorrentData  TorrentDataDTO  `json:"torrentData"`
	TorrentStats TorrentStatsDTO `json:"torrentStats"`
}
