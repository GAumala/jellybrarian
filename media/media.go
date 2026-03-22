package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"jellybrarian/text"
)

// TitleNotFound is returned by SubtitleMovie when no library folder exists for the given title.
var TitleNotFound = errors.New("title not found")

type albumInfo struct {
	artist string
	title  string
	path   string
}

type MediaManager struct {
	MediaDir   string
	LibraryDir string
}

// ListMedia returns entries in the media dir sorted by modification time ascending (oldest first).
// If limit > 0, only the most recent limit entries are returned.
func (mgr MediaManager) ListMedia(limit int) ([]string, error) {
	entries, err := os.ReadDir(mgr.MediaDir)
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

// listJellyfinDirNames returns the names of immediate subdirectories under path, sorted alphabetically.
func listJellyfinDirNames(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir %q: %w", path, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// filterTitlesByQuery returns titles that match all space-separated keywords in q.
// Matching is case-insensitive and accent-insensitive. If q is empty, all titles are returned.
func filterTitlesByQuery(titles []string, q string) []string {
	q = strings.TrimSpace(q)
	if q == "" {
		return titles
	}
	normQuery := text.NormalizeForSearch(q)
	tokens := strings.Fields(normQuery)
	if len(tokens) == 0 {
		return titles
	}
	var out []string
	for _, title := range titles {
		normTitle := text.NormalizeForSearch(title)
		allMatch := true
		for _, tok := range tokens {
			if !strings.Contains(normTitle, tok) {
				allMatch = false
				break
			}
		}
		if allMatch {
			out = append(out, title)
		}
	}
	return out
}

// ListLibraryTitles returns subdirectory names under the selected library dir (movies or TV).
// If q is non-empty, results are filtered by keyword search (case and accent insensitive).
func (mgr MediaManager) ListLibraryTitles(q string) ([]string, error) {
	names, err := listJellyfinDirNames(mgr.LibraryDir)
	if err != nil {
		return nil, err
	}
	return filterTitlesByQuery(names, q), nil
}

// OrganizeArtist finds all directories in the media dir matching "<artist> - <album>"
// and hard-links their contents into LibraryDir/<artist>/<album>/.
// Multi-disc albums (subdirectories instead of files) are placed under Disc <N>/.
func (mgr MediaManager) OrganizeArtist(artist string) ([]string, error) {
	entries, err := os.ReadDir(mgr.MediaDir)
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

		albumPath := filepath.Join(mgr.MediaDir, e.Name())
		info := albumInfo{artist, title, albumPath}
		result, err := mgr.organizeAlbum(info)
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

func (mgr MediaManager) organizeAlbum(album albumInfo) ([]string, error) {
	entries, err := os.ReadDir(album.path)
	if err != nil {
		return nil, err
	}

	hasDiscDirs := false
	for _, e := range entries {
		if e.IsDir() {
			hasDiscDirs = true
			break
		}
	}

	destBase := filepath.Join(mgr.LibraryDir, album.artist, album.title)
	links := make(map[string]string)

	if hasDiscDirs {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			discNum, err := parseDiscNumber(e.Name())
			if err != nil {
				return nil, fmt.Errorf("could not parse disc number from %q: %w", e.Name(), err)
			}
			discDir := filepath.Join(album.path, e.Name())
			discEntries, err := os.ReadDir(discDir)
			if err != nil {
				return nil, err
			}
			destDisc := filepath.Join(destBase, fmt.Sprintf("Disc %d", discNum))
			for _, de := range discEntries {
				if !de.IsDir() {
					links[filepath.Join(discDir, de.Name())] = filepath.Join(destDisc, de.Name())
				}
			}
		}
	} else {
		for _, e := range entries {
			if !e.IsDir() {
				links[filepath.Join(album.path, e.Name())] = filepath.Join(destBase, e.Name())
			}
		}
	}

	return hardlinkFiles(links)
}

// hardlinkFiles creates hard links for each src -> dst pair. Creates parent dirs as needed.
// If a destination already exists, it is removed and replaced by the new hard link.
// On first link failure, returns the list built so far and the error.
func hardlinkFiles(links map[string]string) ([]string, error) {
	var linked []string
	for src, dst := range links {
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return linked, fmt.Errorf("failed to create dir %q: %w", filepath.Dir(dst), err)
		}
		if _, err := os.Stat(dst); err == nil {
			os.Remove(dst)
		}
		if err := os.Link(src, dst); err != nil {
			return linked, fmt.Errorf("failed to link %q -> %q: %w", src, dst, err)
		}
		linked = append(linked, dst)
	}
	return linked, nil
}

// AddTVSeason finds all video files at the given media-path (a file or directory under MediaDir),
// parses season/episode from each filename, and hardlinks them into LibraryDir
// as {title}/Season N/{title} - S01E01.ext so Jellyfin can find them.
// Files that cannot be parsed for episode info are skipped.
func (mgr MediaManager) AddTVSeason(mediaPath string, title string) ([]string, error) {
	srcDir := filepath.Join(mgr.MediaDir, mediaPath)
	videos, err := FindVideoFiles(srcDir)
	if err != nil {
		return nil, err
	}
	links := make(map[string]string)
	for _, srcPath := range videos {
		base := filepath.Base(srcPath)
		info := GetSeasonEpisode(base, false)
		if info == nil {
			continue
		}
		ext := filepath.Ext(srcPath)
		seasonDir := filepath.Join(mgr.LibraryDir, title, fmt.Sprintf("Season %d", info.Season))
		destPath := filepath.Join(seasonDir, fmt.Sprintf("%s - %s%s", title, info.FormatCode(), ext))
		links[srcPath] = destPath
	}
	return hardlinkFiles(links)
}

// AddMovie finds video file(s) at the given media-path (a file or directory under MediaDir),
// plus audio tracks (.XX.aac, .XX.ac3) and subtitles (.XX.srt) with 2-letter language codes,
// and hardlinks them into LibraryDir. Videos: {title}/{title}{ext} or -part-N for
// multiple. Audio/subs: {title}/{title}.{lang}.{ext} (e.g. title.es.aac, title.en.srt).
func (mgr MediaManager) AddMovie(mediaPath string, title string) ([]string, error) {
	srcPath := filepath.Join(mgr.MediaDir, mediaPath)
	info, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("media path %q: %w", srcPath, err)
	}

	videos, err := FindVideoFiles(srcPath)
	if err != nil {
		return nil, err
	}
	if len(videos) == 0 {
		return nil, nil
	}

	movieDir := filepath.Join(mgr.LibraryDir, title)
	links := make(map[string]string)
	for i, src := range videos {
		ext := filepath.Ext(src)
		if len(videos) == 1 {
			links[src] = filepath.Join(movieDir, title+ext)
		} else {
			links[src] = filepath.Join(movieDir, fmt.Sprintf("%s-part-%d%s", title, i+1, ext))
		}
	}
	if info.IsDir() {
		extras, err := findMovieExtraTracks(srcPath)
		if err != nil {
			return nil, fmt.Errorf("scanning for audio/subtitles: %w", err)
		}
		for _, t := range extras {
			var base string
			if t.lang == "" {
				base = fmt.Sprintf("%s.%s", title, t.ext)
			} else {
				base = fmt.Sprintf("%s.%s.%s", title, t.lang, t.ext)
			}
			links[t.path] = filepath.Join(movieDir, base)
		}
	}
	return hardlinkFiles(links)
}

// SubtitleMovie creates a subtitle file for an existing movie in the library.
// The movie directory must exist under LibraryDir/title.
// If lang is non-empty, the file will be named {title}.{lang}.srt, otherwise {title}.srt.
// subs is the full SRT (or other subtitle) text to write.
// Returns the path to the created subtitle file.
func (mgr MediaManager) SubtitleMovie(title, lang, subs string) (string, error) {
	movieDir := filepath.Join(mgr.LibraryDir, title)
	// Check that movie directory exists
	if _, err := os.Stat(movieDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: the provided title %q was not found in the movies library; use the exact movie folder name", TitleNotFound, title)
		}
		return "", fmt.Errorf("cannot access movie directory %q: %w", movieDir, err)
	}

	var filename string
	if lang == "" {
		filename = title + ".srt"
	} else {
		filename = fmt.Sprintf("%s.%s.srt", title, strings.ToLower(lang))
	}
	subPath := filepath.Join(movieDir, filename)

	// Write file, overwriting if exists
	if err := os.WriteFile(subPath, []byte(subs), 0644); err != nil {
		return "", fmt.Errorf("failed to write subtitle file %q: %w", subPath, err)
	}
	return subPath, nil
}
