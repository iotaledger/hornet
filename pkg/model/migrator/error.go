package migrator

// CriticalError is an error which is critical, meaning that migration components no longer can run.
type CriticalError struct {
	err error
}

func (ce CriticalError) Error() string { return ce.err.Error() }
func (ce CriticalError) Unwrap() error { return ce.err }

// SoftError is an error which is soft, meaning that migration components can still run.
type SoftError struct {
	err error
}

func (se SoftError) Error() string { return se.err.Error() }
func (se SoftError) Unwrap() error { return se.err }
