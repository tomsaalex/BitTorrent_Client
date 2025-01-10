package torrentclient

import "github.com/tomsaalex/BitTorrent_Client/backend/customdatatypes"

// Based on: https://wiki.theory.org/BitTorrentSpecification#Metainfo_File_Structure
// TODO: Consider possible extension that supports an announce-list
type TorrentData struct {
	Announce     string
	CreatedBy    string // optional
	CreationDate int    // optional
	Encoding     string // optional
	Comment      string // optional

	PieceLength int
	PieceHashes []customdatatypes.CustomHash // SHA1, 20 bytes each
	Private     int8                         // optional
	Name        string

	// Single File Mode
	FileLength int

	// Multiple File Mode
	Files       []FileData
	TorrentSize int // Not in the specification, just added to not have to calculate it everytime

	// Fields outside metadata file
	Infohash customdatatypes.CustomHash
}

type FileData struct {
	FileLength int
	FilePath   string
}
