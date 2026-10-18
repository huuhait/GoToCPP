package handler

import (
	"errors"

	"example.com/layers/service"
)

type Status int

type httpError struct {
	status Status
	cause  error
}

func (h *httpError) Error() string { return "http error" }
func (h *httpError) Unwrap() []error {
	return []error{h.cause, errors.New("logged")}
}

func (h *httpError) Status() Status { return h.status }

func Handle(id int) error {
	err := service.Load(id)
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrInvalid) {
		return &httpError{400, err}
	}
	return &httpError{500, err}
}
