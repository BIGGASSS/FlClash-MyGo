//go:build !windows

package store

import "os"

func replaceFile(src, dst string) error { return os.Rename(src, dst) }
