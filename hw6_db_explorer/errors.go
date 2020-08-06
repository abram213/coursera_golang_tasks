package main

const (
	InternalErrJSON     = `{"error": "internal server error"}`
	PageNotFoundErrJSON = `{"error": "page not found"}`
)

type SpecificError struct {
	Message string `json:"error"`
	Code    int    `json:"-"`
}

func (se SpecificError) Error() string {
	return se.Message
}
