package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// FinalizedSegmentSize returns the size of the on-disk container after the
// adapter has closed it. Only files inside the recording directory qualify.
func FinalizedSegmentSize(directory, segment string) (int64, error) {
	if !ValidSegmentPath(segment) {
		return 0, fmt.Errorf("invalid recording segment path")
	}
	name, err := filepath.Localize(segment)
	if err != nil {
		return 0, fmt.Errorf("invalid recording segment path")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return 0, fmt.Errorf("open recording directory: %w", segmentFileCause(err))
	}
	defer root.Close()
	info, err := root.Stat(name)
	if err != nil {
		return 0, fmt.Errorf("stat finalized segment: %w", segmentFileCause(err))
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return 0, fmt.Errorf("finalized segment must be a non-empty file")
	}
	return info.Size(), nil
}

func segmentFileCause(err error) error {
	if pathErr, ok := err.(*os.PathError); ok {
		return pathErr.Err
	}
	return err
}
