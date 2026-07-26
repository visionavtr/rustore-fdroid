package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const pendingRemovalsFile = ".rustore-fdroid-pending-removals.json"

func QueueFileRemoval(idx *IndexV1, name string) error {
	name, err := cleanRepoFileName(name)
	if err != nil {
		return err
	}
	for _, pending := range idx.pendingRemovals {
		if pending == name {
			return nil
		}
	}
	idx.pendingRemovals = append(idx.pendingRemovals, name)
	return nil
}

func CommitPendingRemovals(repoPath string) error {
	path := filepath.Join(repoPath, pendingRemovalsFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending removals: %w", err)
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return fmt.Errorf("parse pending removals: %w", err)
	}

	failed := make([]string, 0)
	var failures []error
	for _, name := range names {
		clean, err := cleanRepoFileName(name)
		if err != nil {
			failed = append(failed, name)
			failures = append(failures, err)
			continue
		}
		filePath := filepath.Join(repoPath, clean)
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			failed = append(failed, clean)
			failures = append(failures, fmt.Errorf("remove %s: %w", clean, err))
			continue
		}
		removeEmptyParents(repoPath, filepath.Dir(filePath))
	}

	if len(failed) > 0 {
		if err := savePendingRemovals(repoPath, failed); err != nil {
			failures = append(failures, err)
		}
		return errors.Join(failures...)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pending removals manifest: %w", err)
	}
	return nil
}

func loadPendingRemovals(repoPath string, idx *IndexV1) error {
	data, err := os.ReadFile(filepath.Join(repoPath, pendingRemovalsFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending removals: %w", err)
	}
	if err := json.Unmarshal(data, &idx.pendingRemovals); err != nil {
		return fmt.Errorf("parse pending removals: %w", err)
	}
	for i, name := range idx.pendingRemovals {
		clean, err := cleanRepoFileName(name)
		if err != nil {
			return err
		}
		idx.pendingRemovals[i] = clean
	}
	return nil
}

func savePendingRemovals(repoPath string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	names = append([]string(nil), names...)
	sort.Strings(names)
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(repoPath, pendingRemovalsFile), data, 0o600)
}

func cleanRepoFileName(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid repository file name %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid repository file name %q", name)
	}
	return clean, nil
}

func removeEmptyParents(repoPath, dir string) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return
	}
	for dir != repoPath && strings.HasPrefix(dir, repoPath+string(filepath.Separator)) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
