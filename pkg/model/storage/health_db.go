package storage

const (
	DBVersion = 1
)

func (s *Storage) MarkDatabasesCorrupted() error {

	var markingErr error
	for _, h := range s.healthTrackers {
		if err := h.MarkCorrupted(); err != nil {
			markingErr = err
		}
	}
	return markingErr
}

func (s *Storage) MarkDatabasesTainted() error {

	var markingErr error
	for _, h := range s.healthTrackers {
		if err := h.MarkTainted(); err != nil {
			markingErr = err
		}
	}
	return markingErr
}

func (s *Storage) MarkDatabasesHealthy() error {

	for _, h := range s.healthTrackers {
		if err := h.MarkHealthy(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) AreDatabasesCorrupted() (bool, error) {

	for _, h := range s.healthTrackers {
		corrupted, err := h.IsCorrupted()
		if err != nil {
			return true, err
		}
		if corrupted {
			return true, nil
		}
	}
	return false, nil
}

func (s *Storage) AreDatabasesTainted() (bool, error) {

	for _, h := range s.healthTrackers {
		tainted, err := h.IsTainted()
		if err != nil {
			return true, err
		}
		if tainted {
			return true, nil
		}
	}
	return false, nil
}

func (s *Storage) CheckCorrectDatabasesVersion() (bool, error) {

	for _, h := range s.healthTrackers {
		correct, err := h.CheckCorrectDatabaseVersion()
		if err != nil {
			return false, err
		}
		if !correct {
			return false, nil
		}
	}

	return true, nil
}

// UpdateDatabasesVersion tries to migrate the existing data to the new database version.
func (s *Storage) UpdateDatabasesVersion() (bool, error) {

	allCorrect := true
	for _, h := range s.healthTrackers {
		_, err := h.UpdateDatabaseVersion()
		if err != nil {
			return false, err
		}

		correct, err := h.CheckCorrectDatabaseVersion()
		if err != nil {
			return false, err
		}
		if !correct {
			allCorrect = false
		}
	}

	return allCorrect, nil
}
