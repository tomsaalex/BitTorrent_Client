package torrentclient

type ConnectionDataStatusDTO struct {
	DownloadedData int
	UploadedData   int

	DownloadSpeed int
	UploadSpeed   int

	Peer         peerDTO
	PeerBitfield []byte
}

type TorrentStatsDTO struct {
	TorrentInfohash string

	UploadedBytes   int
	DownloadedBytes int

	ConnectionDataStatuses map[string]ConnectionDataStatusDTO

	PiecesStoredToDisk []byte
	RequestedPieces    []int
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

	newDTO.PiecesStoredToDisk = tStats.piecesStoredToDisk.ExposeBitfield()
	newDTO.RequestedPieces = make([]int, len(tStats.requestedPieces))

	copy(newDTO.RequestedPieces, tStats.requestedPieces)

	newDTO.ConnectionDataStatuses = make(map[string]ConnectionDataStatusDTO)

	for key, val := range tStats.connectionDataStatuses {
		cdsDTO := connectionDataStatusToDTO(&val)

		newDTO.ConnectionDataStatuses[key] = cdsDTO
	}

	newDTO.TorrentInfohash = tStats.torrentInfohash.String()

	return newDTO
}
