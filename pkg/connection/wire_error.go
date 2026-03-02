package connection

import "errors"

// wireError is an unexported deserialization target that captures ALL v2+v3 wire fields.
// It is used as the intermediate representation during RPCError.UnmarshalCBOR,
// before populating the v2 public fields on RPCError and creating a ServerError for the v3 view.
type wireError struct {
	Code        int        `json:"code"`
	Message     string     `json:"message,omitempty"`
	Description string     `json:"description,omitempty"` // SurrealDB v2 only
	Kind        string     `json:"kind,omitempty"`        // SurrealDB v3 only
	Details     any        `json:"details,omitempty"`     // SurrealDB v3 only
	Cause       *wireError `json:"cause,omitempty"`       // SurrealDB v3 only
}

func (w *wireError) Error() string {
	if w == nil {
		return "<nil>"
	}
	return w.Message
}

func (r *wireError) As(err any) bool {
	if r == nil {
		return false
	}
	switch e := err.(type) {
	case *ServerError:
		e.Message = r.Message
		e.Kind = r.Kind
		e.Code = r.Code
		e.Details = r.Details

		var cause ServerError
		if errors.As(r.Cause, &cause) {
			e.Cause = &cause
		}

		return true
	case **ServerError:
		se := &ServerError{
			Message: r.Message,
			Kind:    r.Kind,
			Code:    r.Code,
			Details: r.Details,
		}

		var cause ServerError
		if errors.As(r.Cause, &cause) {
			se.Cause = &cause
		}

		*e = se
		return true
	case *RPCError:
		e.Code = r.Code
		e.Message = r.Message
		e.Description = r.Description
		return true
	case **RPCError:
		rpcErr := &RPCError{
			Code:        r.Code,
			Message:     r.Message,
			Description: r.Description,
		}
		*e = rpcErr
		return true
	}
	return false
}
