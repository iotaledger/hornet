package storage

func NewDatabaseError(cause error) *ErrDatabaseError {
	return &ErrDatabaseError{Inner: cause}
}

type ErrDatabaseError struct {
	Inner error
}

func (e ErrDatabaseError) Cause() error {
	return e.Inner
}

func (e ErrDatabaseError) Error() string {
	return "database error: " + e.Inner.Error()
}
