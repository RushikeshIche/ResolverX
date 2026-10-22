//go:build !windows

// Package singleinstance is a no-op on non-Windows platforms.
// On Linux/macOS the OS process model already prevents duplicate daemon ports.
package singleinstance

import "errors"

// ErrAlreadyRunning is returned when another instance is already active.
var ErrAlreadyRunning = errors.New("ResolverX is already running")

// Acquire is a no-op on non-Windows systems and always succeeds.
func Acquire() error {
	return nil
}
