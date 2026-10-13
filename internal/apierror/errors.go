package apierror

import "fmt"

type APICodes struct {
	HTTPv1 int
}

type Info struct {
	Code    string
	Type    string
	Title   string
	Message string
	Codes   APICodes
}

type Error struct {
	info Info
	err  error
}

func (e *Error) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.info.Message
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) Info() Info {
	return e.info
}

func (e *Error) Msg(message string) *Error {
	e.info.Message = message
	return e
}

func (e *Error) Msgf(format string, args ...any) *Error {
	return e.Msg(fmt.Sprintf(format, args...))
}

func New(code string) *Error {
	return &Error{
		info: fillInfo(code),
	}
}

func Wrap(code string, err error) *Error {
	apiErr := New(code)
	apiErr.err = err
	return apiErr
}

func fillInfo(code string) Info {
	info, ok := infos[code]
	if !ok {
		panic("unknown API error code: " + code)
	}
	info.Code = code
	return info
}
