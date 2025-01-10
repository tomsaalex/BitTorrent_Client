package torrentclient

import "fmt"

type IOError struct {
	Message string
}

func (ioe *IOError) Error() string {
	return fmt.Sprintf("IOError: %s", ioe.Message)
}
