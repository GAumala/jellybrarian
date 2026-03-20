package media

import (
	"fmt"
	"regexp"
	"strconv"
)

// Copied from https://github.com/Phatnoir/jellyfin-renamer/blob/e221b499b04a4d39e3dffe31b4cf56b5551af8fe/renamer/parser.py

// EpisodeInfo holds parsed season and episode numbers from a filename.
type EpisodeInfo struct {
	Season  int
	Episode int
}

// FormatCode returns the episode info as S01E01 style (or S01E001 for episode >= 100).
func (e EpisodeInfo) FormatCode() string {
	if e.Episode >= 100 {
		return fmt.Sprintf("S%02dE%03d", e.Season, e.Episode)
	}
	return fmt.Sprintf("S%02dE%02d", e.Season, e.Episode)
}

// Episode pattern definitions (ordered by priority within category).
var (
	// S01E01, S1E1, etc.
	patternSxxExx = regexp.MustCompile(`(?i)[Ss](\d{1,2})[\s_.-]*[Ee](\d{1,3})`)
	// 1x01, 01x01, etc.
	patternNxNN = regexp.MustCompile(`(?i)(\d{1,2})x(\d{2,3})`)
	// E01, E001, etc. (single season)
	patternExx = regexp.MustCompile(`(?i)[Ee](\d{1,3})`)
	// Anime/fansub: " - 01 [" or " - 01 (" or " - 01."
	patternAnime = regexp.MustCompile(`-\s+(\d{1,3})\s*[\[\(.]`)

	// Anime: "Part 6 - ..." (JoJo-style); capture group is the part/season number.
	patternAnimePart = regexp.MustCompile(`(?i)Part\s+(\d{1,2})\s+-\s+`)
)

// GetSeasonEpisode extracts season and episode numbers from a filename.
//
// filename is the name to parse (with or without extension).
// animeMode: if true, anime pattern (" - 01 [") is tried first; otherwise
// standard TV patterns (SxxExx, NxNN, Exx) are tried first, with anime as fallback.
//
// Pattern priority when animeMode is true:
//  1. Anime pattern (- 01 [)
//  2. SxxExx, NxNN, Exx
//
// Pattern priority when animeMode is false:
//  1. SxxExx, NxNN, Exx
//  2. Anime pattern (fallback)
func GetSeasonEpisode(filename string, animeMode bool) *EpisodeInfo {
	if animeMode {
		if info := matchAnime(filename); info != nil {
			return info
		}
	}

	if info := matchSxxExx(filename); info != nil {
		return info
	}
	if info := matchNxNN(filename); info != nil {
		return info
	}
	if info := matchExx(filename); info != nil {
		return info
	}

	if !animeMode {
		if info := matchAnime(filename); info != nil {
			return info
		}
	}

	return nil
}

func matchSxxExx(s string) *EpisodeInfo {
	m := patternSxxExx.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	season, _ := strconv.Atoi(m[1])
	episode, _ := strconv.Atoi(m[2])
	return &EpisodeInfo{Season: season, Episode: episode}
}

func matchNxNN(s string) *EpisodeInfo {
	m := patternNxNN.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	season, _ := strconv.Atoi(m[1])
	episode, _ := strconv.Atoi(m[2])
	return &EpisodeInfo{Season: season, Episode: episode}
}

func matchExx(s string) *EpisodeInfo {
	m := patternExx.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	episode, _ := strconv.Atoi(m[1])
	return &EpisodeInfo{Season: 1, Episode: episode}
}

func matchAnime(s string) *EpisodeInfo {
	m := patternAnime.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	episode, _ := strconv.Atoi(m[1])

	var season = 1
	m = patternAnimePart.FindStringSubmatch(s)
	if m != nil {
		season, _ = strconv.Atoi(m[1])
	}

	return &EpisodeInfo{Season: season, Episode: episode}
}


var discNumberRe = regexp.MustCompile(`\d+`)
func parseDiscNumber(name string) (int, error) {
	match := discNumberRe.FindString(name)
	if match == "" {
		return 0, fmt.Errorf("no number found in %q", name)
	}
	return strconv.Atoi(match)
}
