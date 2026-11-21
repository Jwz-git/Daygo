package favicon

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	diskIconFileLimit = 1024
	diskIconByteLimit = 32 << 20
)

type diskIconCache struct {
	once  sync.Once
	mu    sync.Mutex
	files int
	bytes int64
}

func (c *diskIconCache) initialize(dir string) {
	c.once.Do(func() {
		c.files, c.bytes = pruneDiskIcons(dir, diskIconFileLimit, diskIconByteLimit)
	})
}

func (c *diskIconCache) write(dir, destination string, data []byte) {
	c.initialize(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	file, err := os.CreateTemp(dir, ".favicon-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil || os.Rename(file.Name(), destination) != nil {
		return
	}
	// Replacements are counted conservatively until a sweep. Prune below the
	// limit so later network misses do not repeatedly scan a full directory.
	c.files++
	c.bytes += int64(len(data))
	if c.files > diskIconFileLimit || c.bytes > diskIconByteLimit {
		c.files, c.bytes = pruneDiskIcons(dir, diskIconFileLimit*3/4, diskIconByteLimit*3/4)
	}
}

type diskIcon struct {
	name string
	at   time.Time
	size int64
}

func pruneDiskIcons(dir string, fileLimit int, byteLimit int64) (int, int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	icons := make([]diskIcon, 0, len(entries))
	var bytes int64
	for _, entry := range entries {
		hash, icon := strings.CutSuffix(entry.Name(), ".ico")
		if !icon || len(hash) != 64 || !entry.Type().IsRegular() {
			continue
		}
		if _, err := hex.DecodeString(hash); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		bytes += info.Size()
		icons = append(icons, diskIcon{entry.Name(), info.ModTime(), info.Size()})
	}
	count := len(icons)
	sort.SliceStable(icons, func(i, j int) bool { return icons[i].at.Before(icons[j].at) })
	for _, icon := range icons {
		if count <= fileLimit && bytes <= byteLimit {
			break
		}
		if err := os.Remove(filepath.Join(dir, icon.name)); err == nil || errors.Is(err, os.ErrNotExist) {
			count--
			bytes -= icon.size
		}
	}
	return count, bytes
}
