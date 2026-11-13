// Copyright 2026 The Sculpt Authors
// Licensed under the Apache License, Version 2.0

package substrate

import (
	"io"
	"os"
	"path/filepath"
)

// stageTree copies treeDir into a private temp directory (Python materialise parity).
func stageTree(prefix, treeDir string) (string, error) {
	staged, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	if err := copyTree(treeDir, staged); err != nil {
		_ = os.RemoveAll(staged)
		return "", err
	}
	return staged, nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if de.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
