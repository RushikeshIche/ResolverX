// Package singleinstance ensures only one copy of ResolverX runs at a time.
// It uses a named Windows mutex as the lock mechanism.
package singleinstance

import (
	"errors"

	"golang.org/x/sys/windows"
)

const mutexName = "Global\\ResolverX_SingleInstance_Mutex"

// ErrAlreadyRunning is returned when another instance is already active.
var ErrAlreadyRunning = errors.New("ResolverX is already running")

var globalMutex windows.Handle

// Acquire tries to create a named global mutex.
// Returns ErrAlreadyRunning if another instance already holds it.
// The mutex is automatically released when the process exits.
func Acquire() error {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return err
	}

	handle, err := windows.CreateMutex(nil, true, name)
	if err != nil {
		if err == windows.ERROR_ALREADY_EXISTS {
			return ErrAlreadyRunning
		}
		return err
	}

	// Store globally so it is not garbage-collected before process exit.
	globalMutex = handle
	return nil
}
