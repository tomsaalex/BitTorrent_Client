package torrentclient

import "fmt"

type MalformedTorrentError struct {
	Message string
}

func (tpe *MalformedTorrentError) Error() string {
	return fmt.Sprintf("MalformedTorrentError: %s", tpe.Message)
}
