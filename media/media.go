package media

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"jellybrarian/config"
)

// List returns entries in the media dir sorted by modification time ascending (oldest first).
// If limit > 0, only the most recent limit entries are returned.
func List(dirs config.Directories, limit int) ([]string, error) {
	entries, err := os.ReadDir(dirs.Media)
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

	if limit > 0 && limit < len(names) {
		names = names[len(names)-limit:]
	}

	return names, nil
}

var discNumberRe = regexp.MustCompile(`\d+`)
type albumDir struct {
		artist  string
		title   string
		path    string
}

// OrganizeArtist finds all directories in the media dir matching "<artist> - <album>"
// and hard-links their contents into JellyfinMusic/<artist>/<album>/.
// Multi-disc albums (subdirectories instead of files) are placed under Disc <N>/.
func OrganizeArtist(dirs config.Directories, artist string) ([]string, error) {
	entries, err := os.ReadDir(dirs.Media)
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

		// Parse album name: everything after "<artist> - "
		title := strings.TrimSpace(e.Name()[len(prefix):])
		if title == "" {
			continue
		}

		albumPath := filepath.Join(dirs.Media, e.Name())
		albumDir := albumDir{artist, title, albumPath}
		result, err := organizeAlbum(dirs, albumDir)
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

func organizeAlbum(dirs config.Directories, album albumDir) ([]string, error) {
	entries, err := os.ReadDir(album.path)
	if err != nil {
		return nil, err
	}

	hasFiles := false
	hasDirs := false
	for _, e := range entries {
		if e.IsDir() {
			hasDirs = true
		} else {
			hasFiles = true
		}
	}

	destBase := filepath.Join(dirs.JellyfinMusic, album.artist, album.title)
	var linked []string

	if hasFiles && !hasDirs {
		// Single disc: link files directly into the album dir
		result, err := hardlinkFiles(album.path, destBase, entries)
		if err != nil {
			return nil, err
		}
		linked = append(linked, result...)
	} else if hasDirs && !hasFiles {
		// Multi-disc: each subdir is a disc
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			discNum, err := parseDiscNumber(e.Name())
			if err != nil {
				return linked, fmt.Errorf("could not parse disc number from %q: %w", e.Name(), err)
			}

			discDir := filepath.Join(album.path, e.Name())
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
		// Mixed: treat files as single disc, skip subdirectories
		var fileEntries []os.DirEntry
		for _, e := range entries {
			if !e.IsDir() {
				fileEntries = append(fileEntries, e)
			}
		}
		result, err := hardlinkFiles(album.path, destBase, fileEntries)
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
