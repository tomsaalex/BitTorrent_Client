package generalerrors

import "fmt"

type IllegalAccessError struct {
	Message string
}

func (iae *IllegalAccessError) Error() string {
	return fmt.Sprintf("IllegalAccessError: %s", iae.Message)
}
