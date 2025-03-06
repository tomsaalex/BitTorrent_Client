package torrentclient

type ConnectionDataStatusDTO struct {
	DownloadedData int `json:"downloadedData"`
	UploadedData   int `json:"uploadedData"`

	DownloadSpeed int `json:"downloadSpeed"`
	UploadSpeed   int `json:"uploadSpeed"`

	Peer         peerDTO `json:"peerDTO"`
	PeerBitfield []byte  `json:"peerBitfield"`
}

type TorrentStatsDTO struct {
	UploadedBytes        int          `json:"uploadedBytes"`
	DownloadedBytes      int          `json:"downloadedBytes"`
	State                TorrentState `json:"torrentState"`
	RecheckedPiecesCount int          `json:"recheckedPiecesCount"`

	ConnectionDataStatuses map[string]ConnectionDataStatusDTO `json:"connectionDataStatuses"`

	NumberOfPiecesOnDisk int    `json:"piecesOnDiskCount"`
	PiecesStoredToDisk   []byte `json:"piecesStoredToDisk"`
	RequestedPieces      []int  `json:"requestedPieces"`
}

func connectionDataStatusToDTO(cds *connectionDataStatus) ConnectionDataStatusDTO {
	newDTO := ConnectionDataStatusDTO{}

	newDTO.DownloadedData = cds.downloadedData
	newDTO.UploadedData = cds.uploadedData

	newDTO.DownloadSpeed = cds.downloadSpeed
	newDTO.UploadSpeed = cds.uploadSpeed

	newDTO.Peer = peerDTO{}
	newDTO.Peer.Ip = cds.peer.ip
	newDTO.Peer.Port = cds.peer.port
	newDTO.Peer.PeerID = cds.peer.peerID.String()

	newDTO.PeerBitfield = cds.peerBitfield.ExposeBitfield()

	return newDTO
}

func torrentStatsSnapshot(tStats *TorrentStats) TorrentStatsDTO {
	// TODO: Not technically a perfect snapshot since the pieces bitfield can still update during this, but that's not super important.
	// TODO: Should this be moved to the torrentStats?
	newDTO := TorrentStatsDTO{}

	tStats.requestedPiecesMu.Lock()
	defer tStats.requestedPiecesMu.Unlock()

	tStats.connectionDataStatusesMu.Lock()
	defer tStats.connectionDataStatusesMu.Unlock()

	tStats.uploadedBytesMu.Lock()
	defer tStats.uploadedBytesMu.Unlock()

	tStats.downloadedBytesMu.Lock()
	defer tStats.downloadedBytesMu.Unlock()

	tStats.stateMu.Lock()
	defer tStats.stateMu.Unlock()

	tStats.recheckedPiecesCountMu.Lock()
	defer tStats.recheckedPiecesCountMu.Unlock()

	newDTO.UploadedBytes = tStats.uploadedBytes
	newDTO.DownloadedBytes = tStats.downloadedBytes
	newDTO.RecheckedPiecesCount = tStats.recheckedPiecesCount
	newDTO.State = tStats.state

	newDTO.NumberOfPiecesOnDisk = tStats.piecesStoredToDisk.BitsSetCount()
	newDTO.PiecesStoredToDisk = tStats.piecesStoredToDisk.ExposeBitfield()
	newDTO.RequestedPieces = make([]int, len(tStats.requestedPieces))

	copy(newDTO.RequestedPieces, tStats.requestedPieces)

	newDTO.ConnectionDataStatuses = make(map[string]ConnectionDataStatusDTO)

	for key, val := range tStats.connectionDataStatuses {
		cdsDTO := connectionDataStatusToDTO(&val)

		newDTO.ConnectionDataStatuses[key] = cdsDTO
	}

	return newDTO
}
