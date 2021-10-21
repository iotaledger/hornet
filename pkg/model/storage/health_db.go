package storage

const (
	DBVersion = 1
)

func (s *Storage) MarkDatabasesCorrupted() error {

	for _, h := range s.healthTrackers {
		err := h.markCorrupted()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) MarkDatabasesTainted() error {

	for _, h := range s.healthTrackers {
		err := h.markTainted()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) MarkDatabasesHealthy() error {

	for _, h := range s.healthTrackers {
		err := h.markHealthy()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) AreDatabasesCorrupted() (bool, error) {

	for _, h := range s.healthTrackers {
		corrupted, err := h.isCorrupted()
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
		tainted, err := h.isTainted()
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
		correct, err := h.checkCorrectDatabaseVersion(DBVersion)
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
		_, err := h.updateDatabaseVersion()
		if err != nil {
			return false, err
		}

		correct, err := h.checkCorrectDatabaseVersion(DBVersion)
		if err != nil {
			return false, err
		}
		if !correct {
			allCorrect = false
		}
	}

	return allCorrect, nil
}
