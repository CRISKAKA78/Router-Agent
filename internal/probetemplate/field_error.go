package probetemplate

// FieldError preserves ErrInvalid while exposing an actionable public field.
type FieldError struct {
	Field  string
	Detail string
}

func (e *FieldError) Error() string { return e.Detail }
func (e *FieldError) Unwrap() error { return ErrInvalid }
