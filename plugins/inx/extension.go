package inx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/iotaledger/hive.go/configuration"
)

const (
	INXManifestName       = "name"
	INXManifestEntrypoint = "entrypoint"
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

	config := configuration.New()
	if err := config.LoadFile(filepath.Join(absPath, "inx.json")); err != nil {
		return nil, err
	}
	if len(config.String(INXManifestName)) == 0 {
		return nil, fmt.Errorf("inx.json missing key %s", INXManifestName)
	}
	if len(config.String(INXManifestEntrypoint)) == 0 {
		return nil, fmt.Errorf("inx.json missing key %s", INXManifestEntrypoint)
	}

	return &Extension{
		Path:       absPath,
		Name:       config.String(INXManifestName),
		Entrypoint: config.String(INXManifestEntrypoint),
	}, nil
}

func (e *Extension) Start(inxPort int) error {
	e.cmd = exec.Command(e.Entrypoint)
	e.cmd.Env = append(syscall.Environ(), fmt.Sprintf("INX_PORT=%d", inxPort))
	e.cmd.Dir = e.Path

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
