package inx

import (
	"os/exec"
	"path/filepath"
)

type Extension struct {
	Name       string
	Entrypoint string
	cmd        *exec.Cmd
}

func NewExtension(path string) *Extension {
	//TODO: read inx.json or some config file
	return &Extension{
		Name:       filepath.Base(path),
		Entrypoint: filepath.Join(path, "run.sh"),
	}
}

func (e *Extension) Run() error {
	e.cmd = exec.Command(e.Entrypoint)
	return e.cmd.Run()
}

func (e *Extension) Kill() error {
	if e.cmd != nil {
		return e.cmd.Process.Kill()
	}
	return nil
}
