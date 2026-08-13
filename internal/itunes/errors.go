package itunes

import "fmt"

type httpStatusError struct {
	operation  string
	statusCode int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("%s request returned status %d", e.operation, e.statusCode)
}

func (e *httpStatusError) HTTPStatusCode() int {
	return e.statusCode
}
