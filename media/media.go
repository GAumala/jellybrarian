package media

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// List returns entries in dir sorted by modification time ascending (oldest first).
func List(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir: %w", err)
	}

	type entry struct {
		name    string
		modTime int64
	}

	items := make([]entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, entry{name: e.Name(), modTime: info.ModTime().UnixNano()})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime < items[j].modTime
	})

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.name
	}
	return names, nil
}

var discNumberRe = regexp.MustCompile(`\d+`)

// OrganizeArtist finds all directories in mediaDir matching "<artist> - <album>"
// and hard-links their contents into jellyfinMusicDir/<artist>/<album>/.
// Multi-disc albums (subdirectories instead of files) are placed under Disc <N>/.
func OrganizeArtist(mediaDir, jellyfinMusicDir, artist string) ([]string, error) {
	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read media dir: %w", err)
	}

	prefix := strings.ToLower(artist) + " - "
	var organized []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(e.Name()), prefix) {
			continue
		}

		// Parse album name from "<Artist> - <Album>"
		album := e.Name()[len(prefix):]
		if album == "" {
			continue
		}

		srcDir := filepath.Join(mediaDir, e.Name())
		result, err := organizeAlbum(srcDir, jellyfinMusicDir, artist, album)
		if err != nil {
			return organized, fmt.Errorf("failed to organize %q: %w", e.Name(), err)
		}
		organized = append(organized, result...)
	}

	if len(organized) == 0 {
		return nil, fmt.Errorf("no directories found matching artist %q", artist)
	}

	return organized, nil
}

func organizeAlbum(srcDir, jellyfinMusicDir, artist, album string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	// Determine if this is a multi-disc album (contains only directories)
	hasFiles := false
	hasDirs := false
	for _, e := range entries {
		if e.IsDir() {
			hasDirs = true
		} else {
			hasFiles = true
		}
	}

	destBase := filepath.Join(jellyfinMusicDir, artist, album)
	var linked []string

	if hasFiles && !hasDirs {
		// Single disc: link files directly
		result, err := hardlinkFiles(srcDir, destBase, entries)
		if err != nil {
			return nil, err
		}
		linked = append(linked, result...)
	} else if hasDirs && !hasFiles {
		// Multi-disc: iterate subdirectories
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			discNum, err := parseDiscNumber(e.Name())
			if err != nil {
				return linked, fmt.Errorf("could not parse disc number from %q: %w", e.Name(), err)
			}

			discDir := filepath.Join(srcDir, e.Name())
			discEntries, err := os.ReadDir(discDir)
			if err != nil {
				return linked, err
			}

			destDisc := filepath.Join(destBase, fmt.Sprintf("Disc %d", discNum))
			result, err := hardlinkFiles(discDir, destDisc, discEntries)
			if err != nil {
				return linked, err
			}
			linked = append(linked, result...)
		}
	} else {
		// Mixed: treat files as single disc, ignore subdirectories
		var fileEntries []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() {
				fileEntries = append(fileEntries, e)
			}
		}
		result, err := hardlinkFiles(srcDir, destBase, fileEntries)
		if err != nil {
			return nil, err
		}
		linked = append(linked, result...)
	}

	return linked, nil
}

func hardlinkFiles(srcDir, destDir string, entries []os.DirEntry) ([]string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create dir %q: %w", destDir, err)
	}

	var linked []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(destDir, e.Name())

		// Skip if destination already exists
		if _, err := os.Stat(dst); err == nil {
			linked = append(linked, dst+" (already exists)")
			continue
		}

		if err := os.Link(src, dst); err != nil {
			return linked, fmt.Errorf("failed to link %q -> %q: %w", src, dst, err)
		}
		linked = append(linked, dst)
	}

	return linked, nil
}

func parseDiscNumber(name string) (int, error) {
	match := discNumberRe.FindString(name)
	if match == "" {
		return 0, fmt.Errorf("no number found in %q", name)
	}
	return strconv.Atoi(match)
}
