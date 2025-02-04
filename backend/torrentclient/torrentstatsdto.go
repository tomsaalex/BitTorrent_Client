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
	UploadedBytes   int `json:"uploadedBytes"`
	DownloadedBytes int `json:"downloadedBytes"`

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
	newDTO.Peer.ip = cds.peer.ip
	newDTO.Peer.port = cds.peer.port
	newDTO.Peer.peerID = cds.peer.peerID.String()

	newDTO.PeerBitfield = cds.peerBitfield.ExposeBitfield()

	return newDTO
}

func torrentStatsToDTO(tStats *TorrentStats) TorrentStatsDTO {
	newDTO := TorrentStatsDTO{}

	newDTO.UploadedBytes = tStats.uploadedBytes
	newDTO.DownloadedBytes = tStats.downloadedBytes

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
