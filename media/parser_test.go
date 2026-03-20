package media

import (
	"testing"
)

// Copied from https://github.com/Phatnoir/jellyfin-renamer/blob/e221b499b04a4d39e3dffe31b4cf56b5551af8fe/Tests/test_parser.py

func TestGetSeasonEpisode_StandardSxxExx(t *testing.T) {
	tests := []struct {
		filename        string
		expectedSeason  int
		expectedEpisode int
	}{
		{"Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv", 1, 1},
		{"Doctor.Who.2005.S05E04.Time.Of.The.Angels.HDTV.XviD-FoV.avi", 5, 4},
		{"The.Office.S02E15.1080p.BluRay.mkv", 2, 15},
		{"show.s1e5.title.mkv", 1, 5},
		{"Show.S01.E01.Title.mkv", 1, 1}, // space between S and E
		{"Show.S01-E01.Title.mkv", 1, 1}, // dash between S and E
		{"Show.S01_E01.Title.mkv", 1, 1}, // underscore between S and E
		{"Show S10E100.mkv", 10, 100},    // 3-digit episode
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetSeasonEpisode(tt.filename, false)
			if result == nil {
				t.Fatalf("failed to parse: %s", tt.filename)
			}
			if result.Season != tt.expectedSeason || result.Episode != tt.expectedEpisode {
				t.Errorf("%s -> got S%dE%d, want S%dE%d", tt.filename,
					result.Season, result.Episode, tt.expectedSeason, tt.expectedEpisode)
			}
		})
	}
}

func TestGetSeasonEpisode_NxNNFormat(t *testing.T) {
	tests := []struct {
		filename        string
		expectedSeason  int
		expectedEpisode int
	}{
		{"Show.1x01.Title.mkv", 1, 1},
		{"Show.01x05.Title.mkv", 1, 5},
		{"Show 2x15 - Episode Title.avi", 2, 15},
		{"show.10x01.finale.mkv", 10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetSeasonEpisode(tt.filename, false)
			if result == nil {
				t.Fatalf("failed to parse: %s", tt.filename)
			}
			if result.Season != tt.expectedSeason || result.Episode != tt.expectedEpisode {
				t.Errorf("%s -> got S%dE%d, want S%dE%d", tt.filename,
					result.Season, result.Episode, tt.expectedSeason, tt.expectedEpisode)
			}
		})
	}
}

func TestGetSeasonEpisode_SingleSeasonEPattern(t *testing.T) {
	tests := []struct {
		filename       string
		expectedEpisode int
	}{
		{"TestShow.E01.mkv", 1},
		{"TestShow.E005.mkv", 5},
		{"TestShow.E010.mkv", 10},
		{"TestShow.E001.mkv", 1},
		{"ShowName.e05.1080p.WEB-DL.mkv", 5},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetSeasonEpisode(tt.filename, false)
			if result == nil {
				t.Fatalf("failed to parse: %s", tt.filename)
			}
			if result.Season != 1 {
				t.Errorf("%s -> season want 1, got %d", tt.filename, result.Season)
			}
			if result.Episode != tt.expectedEpisode {
				t.Errorf("%s -> episode want %d, got %d", tt.filename, tt.expectedEpisode, result.Episode)
			}
		})
	}
}

func TestGetSeasonEpisode_AnimeFansub(t *testing.T) {
	tests := []struct {
		filename       string
		expectedEpisode int
	}{
		{"[Erai-raws] Cyberpunk - Edgerunners - 01 [1080p][Multiple Subtitle].mkv", 1},
		{"[SubsPlease] Spy x Family - 05 (1080p) [ABC123].mkv", 5},
		{"Show Name - 12 [720p].mkv", 12},
		{"[HorribleSubs] My Hero Academia - 100 [1080p].mkv", 100},
		{"Anime Title - 03 (BD 1080p).mkv", 3},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetSeasonEpisode(tt.filename, true)
			if result == nil {
				t.Fatalf("failed to parse: %s", tt.filename)
			}
			if result.Season != 1 {
				t.Errorf("%s -> season want 1, got %d", tt.filename, result.Season)
			}
			if result.Episode != tt.expectedEpisode {
				t.Errorf("%s -> episode want %d, got %d", tt.filename, tt.expectedEpisode, result.Episode)
			}
		})
	}
}

func TestGetSeasonEpisode_AnimeFallbackWithoutAnimeMode(t *testing.T) {
	filename := "[Erai-raws] Show - 01 [1080p].mkv"
	result := GetSeasonEpisode(filename, false)
	if result == nil {
		t.Fatalf("failed to parse: %s (anime pattern should match as fallback)", filename)
	}
	if result.Season != 1 || result.Episode != 1 {
		t.Errorf("%s -> got S%dE%d, want S01E01", filename, result.Season, result.Episode)
	}
}

func TestGetSeasonEpisode_NoMatch(t *testing.T) {
	filenames := []string{
		"random_file.mkv",
		"movie.2020.1080p.mkv",
		"some.documentary.mkv",
	}
	for _, filename := range filenames {
		t.Run(filename, func(t *testing.T) {
			result := GetSeasonEpisode(filename, false)
			if result != nil {
				t.Errorf("%s -> should not match but got S%dE%d", filename, result.Season, result.Episode)
			}
		})
	}
}

func TestEpisodeInfo_FormatCode(t *testing.T) {
	tests := []struct {
		info     EpisodeInfo
		expected string
	}{
		{EpisodeInfo{1, 1}, "S01E01"},
		{EpisodeInfo{10, 100}, "S10E100"},
		{EpisodeInfo{2, 15}, "S02E15"},
		{EpisodeInfo{1, 5}, "S01E05"},
	}
	for _, tt := range tests {
		got := tt.info.FormatCode()
		if got != tt.expected {
			t.Errorf("EpisodeInfo%+v.FormatCode() = %q, want %q", tt.info, got, tt.expected)
		}
	}
}
