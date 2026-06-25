// Package clierr maps failures to typed process exit codes so scripts and AI
// agents can branch on the kind of failure without parsing stderr. Domain
// packages wrap their errors with the constructors here; main translates the
// wrapped error into the exit code via Code.
//
// Codes follow the convention used by similar agent-native CLIs:
//
//	0  success
//	1  unclassified error
//	2  usage (bad flag or argument)
//	3  not found
//	4  auth (missing or rejected credentials)
//	5  upstream (provider returned an error)
//	7  rate limited
package clierr

import "errors"

// Error carries an exit Code alongside the underlying error. The wrapped
// message is surfaced verbatim, so wrapping never pollutes user-facing text.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}

func Usage(err error) error       { return wrap(2, err) }
func NotFound(err error) error    { return wrap(3, err) }
func Auth(err error) error        { return wrap(4, err) }
func Upstream(err error) error    { return wrap(5, err) }
func RateLimited(err error) error { return wrap(7, err) }

// Code returns the exit code for err: 0 for nil, the wrapped code for a
// *clierr.Error anywhere in the chain, and 1 for anything else.
func Code(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 1
}
