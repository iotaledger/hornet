package inx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Extension struct {
	Path       string
	Name       string
	Entrypoint string
	cmd        *exec.Cmd
}

func NewExtension(path string) (*Extension, error) {
	//TODO: read inx.json or some config file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return &Extension{
		Path:       absPath,
		Name:       filepath.Base(absPath),
		Entrypoint: filepath.Join(absPath, "run.sh"),
	}, nil
}

func (e *Extension) Start() error {
	e.cmd = exec.Command(e.Entrypoint)

	var logFile *os.File
	logFile, err := os.Create(filepath.Join(e.Path, "output.log"))
	if err != nil {
		return fmt.Errorf("unable to open log file: %w", err)
	}
	defer func() { _ = logFile.Close() }()
	e.cmd.Stdout = logFile

	var errFile *os.File
	errFile, err = os.Create(filepath.Join(e.Path, "err.log"))
	if err != nil {
		return fmt.Errorf("unable to open log file: %w", err)
	}
	defer func() { _ = errFile.Close() }()
	e.cmd.Stderr = errFile

	return e.cmd.Start()
}

func (e *Extension) Stop() error {
	if e.cmd != nil && e.cmd.Process != nil {
		return e.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (e *Extension) Kill() error {
	if e.cmd != nil && e.cmd.Process != nil {
		return e.cmd.Process.Kill()
	}
	return nil
}
