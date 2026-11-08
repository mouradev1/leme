//go:build !darwin && !windows && !linux

package secrets

func platformStore() Store { return &fileStore{} }
