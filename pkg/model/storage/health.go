package storage

func (s *Storage) MarkStoresCorrupted() error {

	var markingErr error
	for _, h := range s.healthTrackers {
		if err := h.MarkCorrupted(); err != nil {
			markingErr = err
		}
	}

	return markingErr
}

func (s *Storage) MarkStoresTainted() error {

	var markingErr error
	for _, h := range s.healthTrackers {
		if err := h.MarkTainted(); err != nil {
			markingErr = err
		}
	}

	return markingErr
}

func (s *Storage) MarkStoresHealthy() error {

	for _, h := range s.healthTrackers {
		if err := h.MarkHealthy(); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) AreStoresCorrupted() (bool, error) {

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

func (s *Storage) AreStoresTainted() (bool, error) {

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

func (s *Storage) CheckCorrectStoresVersion() (bool, error) {

	for _, h := range s.healthTrackers {
		correct, err := h.CheckCorrectStoreVersion()
		if err != nil {
			return false, err
		}
		if !correct {
			return false, nil
		}
	}

	return true, nil
}

// UpdateStoresVersion tries to migrate the existing data to the new store version.
func (s *Storage) UpdateStoresVersion() (bool, error) {

	allCorrect := true
	for _, h := range s.healthTrackers {
		_, err := h.UpdateStoreVersion()
		if err != nil {
			return false, err
		}

		correct, err := h.CheckCorrectStoreVersion()
		if err != nil {
			return false, err
		}
		if !correct {
			allCorrect = false
		}
	}

	return allCorrect, nil
}
