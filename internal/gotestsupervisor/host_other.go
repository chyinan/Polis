//go:build !linux

// pattern: Imperative Shell
package gotestsupervisor

import "context"

func readRootQualification(string) ([]byte, error)   { return nil, ErrInvalid }
func checkSystemdUnit(context.Context, string) error { return ErrInvalid }
