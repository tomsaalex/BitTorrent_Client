package torrentclient

import "fmt"

type InvalidMessageError struct {
	Message string
	MsgType string
}

func (ime *InvalidMessageError) Error() string {
	return fmt.Sprintf("InvalidMessageError. MessageType: %s. Reason for invalidity: %s ", ime.MsgType, ime.Message)
}
