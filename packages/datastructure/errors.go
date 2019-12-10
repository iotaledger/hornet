package datastructure

import (
	"github.com/pkg/errors"
)

var (
	ErrNoSuchElement   = errors.New("element does not exist")
	ErrInvalidArgument = errors.New("invalid argument")
)
