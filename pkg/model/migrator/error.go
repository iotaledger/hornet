package migrator

// CriticalError is an error which is critical, meaning that migration components no longer can run.
type CriticalError struct {
	Err error
}

func (ce CriticalError) Error() string {
	return ce.Err.Error()
}

// SoftError is an error which is soft, meaning that migration components can still run.
type SoftError struct {
	Err error
}

func (se SoftError) Error() string {
	return se.Err.Error()
}
