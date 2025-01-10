package bparserrs

import "fmt"

type EncodingError struct {
	Message string
}

type DecodingError struct {
	Message string
}

func (ee *EncodingError) Error() string {
	return fmt.Sprintf("EncodingError: %s", ee.Message)
}

func (ee *DecodingError) Error() string {
	return fmt.Sprintf("DecodingError: %s", ee.Message)
}
