package comm_errors

import "fmt"

type TrackerConnectionError struct {
	Message string
}

func (tce *TrackerConnectionError) Error() string {
	return fmt.Sprintf("MalformedTorrentError: %s", tce.Message)
}
