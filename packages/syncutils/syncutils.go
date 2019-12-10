package syncutils

import (
	"sync"
)

type Mutex sync.Mutex
type RWMutex sync.RWMutex

func (m *Mutex) Lock()   { (*sync.Mutex)(m).Lock() }
func (m *Mutex) Unlock() { (*sync.Mutex)(m).Unlock() }

func (m *RWMutex) Lock()    { (*sync.RWMutex)(m).Lock() }
func (m *RWMutex) Unlock()  { (*sync.RWMutex)(m).Unlock() }
func (m *RWMutex) RLock()   { (*sync.RWMutex)(m).RLock() }
func (m *RWMutex) RUnlock() { (*sync.RWMutex)(m).RUnlock() }

/*
import (
	"time"

	"github.com/sasha-s/go-deadlock"
)

type Mutex deadlock.Mutex
type RWMutex deadlock.RWMutex

func init() {
	deadlock.Opts.DeadlockTimeout = time.Duration(20 * time.Second)
}

func (m *Mutex) Lock()   { (*deadlock.Mutex)(m).Lock() }
func (m *Mutex) Unlock() { (*deadlock.Mutex)(m).Unlock() }

func (m *RWMutex) Lock()    { (*deadlock.RWMutex)(m).Lock() }
func (m *RWMutex) Unlock()  { (*deadlock.RWMutex)(m).Unlock() }
func (m *RWMutex) RLock()   { (*deadlock.RWMutex)(m).RLock() }
func (m *RWMutex) RUnlock() { (*deadlock.RWMutex)(m).RUnlock() }
*/
