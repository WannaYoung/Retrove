package scrape

import "fmt"

// Kind classifies ScreenScraper failures for jobs / UI.
type Kind string

const (
	KindNetwork  Kind = "network"
	KindAuth     Kind = "auth"
	KindQuota    Kind = "quota"
	KindNotFound Kind = "not_found"
	KindConfig   Kind = "config"
	KindParse    Kind = "parse"
	KindMedia    Kind = "media"
	KindInternal Kind = "internal"
)

type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func classify(kind Kind, msg string, cause error) *Error {
	return &Error{Kind: kind, Message: msg, Cause: cause}
}

func AsKind(err error) Kind {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Kind
	}
	return KindInternal
}
