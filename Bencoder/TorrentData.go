package Bencoding

// Based on: https://wiki.theory.org/BitTorrentSpecification#Metainfo_File_Structure
// TODO: Consider possible extension that supports an announce-list
type TorrentData struct {
	Announce     string
	CreatedBy    string // optional
	CreationDate int    // optional
	Encoding     string // optional
	Comment      string // optional

	PieceLength int
	PieceHashes []string // SHA1, 20 bytes each
	Private     int8     // optional
	Name        string

	// Single File Mode
	FileLength int

	// Multiple File Mode
	Files []FileData
}

type FileData struct {
	FileLength int
	FilePath   string
}
