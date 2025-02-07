package torrentclient

type TorrentDataDTO struct {
	Announce     string `json:"announce"`
	CreatedBy    string `json:"createdBy"`    // optional
	CreationDate int    `json:"creationDate"` // optional
	Encoding     string `json:"encoding"`     // optional
	Comment      string `json:"comment"`      // optional

	PieceLength int    `json:"pieceLength"`
	PieceNumber int    `json:"pieceNumber"`
	Private     bool   `json:"private"`
	Name        string `json:"name"`

	// Single File Mode
	FileLength int `json:"fileLength"`

	// Multiple File Mode
	Files       []FileDataDTO `json:"files"`
	TorrentSize int           `json:"torrentSize"` // Not in the specification, just added to not have to calculate it everytime

	// Fields outside metadata file
	Infohash string `json:"infohash"`
}

type FileDataDTO struct {
	FileLength int    `json:"fileLength"`
	FilePath   string `json:"filePath"`
}

func fileDataToDTO(fd *FileData) FileDataDTO {
	newDTO := FileDataDTO{}

	newDTO.FileLength = fd.FileLength
	newDTO.FilePath = fd.FilePath

	return newDTO
}

func torrentDataToDTO(td *TorrentData) TorrentDataDTO {
	newDTO := TorrentDataDTO{}

	newDTO.Announce = td.Announce
	newDTO.CreatedBy = td.CreatedBy
	newDTO.CreationDate = td.CreationDate
	newDTO.Encoding = td.Encoding
	newDTO.Comment = td.Comment
	newDTO.PieceLength = td.PieceLength
	newDTO.PieceNumber = len(td.PieceHashes)
	newDTO.Private = td.Private == 1
	newDTO.Name = td.Name

	newDTO.FileLength = td.FileLength

	for _, fd := range td.Files {
		fdDTO := fileDataToDTO(&fd)
		newDTO.Files = append(newDTO.Files, fdDTO)
	}

	newDTO.TorrentSize = td.TorrentSize
	newDTO.Infohash = td.Infohash.String()

	return newDTO
}
