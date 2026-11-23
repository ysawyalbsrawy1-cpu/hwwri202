//go:build !linux

package main

// raiseNofileLimit is a no-op on non-Linux platforms.
func raiseNofileLimit() {}
