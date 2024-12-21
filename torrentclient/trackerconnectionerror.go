package torrentclient

import "fmt"

type TrackerConnectionError struct {
	Message string
}

func (tce *TrackerConnectionError) Error() string {
	return fmt.Sprintf("TrackerConnectionError: %s", tce.Message)
}
