package generalerrors

import "fmt"

type TypeInitializationError struct {
	TypeName     string
	ErrorDetails string
}

func (tie *TypeInitializationError) Error() string {
	return fmt.Sprintf("TypeInitializationError: Couldn't initialize type %s. Reason: %s", tie.TypeName, tie.ErrorDetails)
}
