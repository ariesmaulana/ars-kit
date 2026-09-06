package upload

import (
	"errors"
	"fmt"
)

// Sentinel errors categorize why an upload failed. Callers can branch on
// either form:
//
//	if errors.Is(err, upload.ErrInvalidMIME) { ... }
//	if ue, ok := err.(*UploadError); ok && ue.Code == upload.ErrTooLarge { ... }
var (
	ErrBadRequest  = errors.New("bad_request")
	ErrInvalidMIME = errors.New("invalid_mime")
	ErrTooLarge    = errors.New("too_large")
	ErrTransport   = errors.New("transport")
	ErrTimeout     = errors.New("timeout")
)

// UploadError wraps a failed operation with a code, backend, op, and cause.
type UploadError struct {
	// Code is one of the package sentinels (ErrBadRequest, ...).
	Code    error
	Backend string
	Op      string
	Err     error
}

func (e *UploadError) Error() string {
	if e.Backend != "" {
		return fmt.Sprintf("%s/%s: %v", e.Backend, e.Op, e.Err)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return e.Err.Error()
}

func (e *UploadError) Unwrap() error {
	return e.Err
}

// wrapError creates an UploadError whose Code and unwrap chain carry the
// sentinel, so both errors.Is and .Code-based switching work.
func wrapError(code error, backend, op string, err error) error {
	if err == nil {
		return nil
	}
	return &UploadError{
		Code:    code,
		Backend: backend,
		Op:      op,
		Err:     errors.Join(code, err),
	}
}
