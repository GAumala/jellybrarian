package media

import (
	"os"
	"path/filepath"
	"testing"

	"jellybrarian/config"
)

// testDirs creates a temporary directory structure and returns a config.Directories
// pointing to it. Cleaned up automatically when the test finishes.
func testDirs(t *testing.T) config.Directories {
	t.Helper()
	root := t.TempDir()

	dirs := config.Directories{
		Media:          filepath.Join(root, "media"),
		JellyfinMusic:  filepath.Join(root, "jellyfin", "music"),
		JellyfinMovies: filepath.Join(root, "jellyfin", "movies"),
		JellyfinTV:     filepath.Join(root, "jellyfin", "tv"),
	}

	for _, d := range []string{dirs.Media, dirs.JellyfinMusic, dirs.JellyfinMovies, dirs.JellyfinTV} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	return dirs
}

// createFile creates a file with the given content inside dir.
func createFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", name, err)
	}
}

func TestList_Empty(t *testing.T) {
	dirs := testDirs(t)

	names, err := List(dirs, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty list, got %v", names)
	}
}

func TestList_ReturnsEntries(t *testing.T) {
	dirs := testDirs(t)

	// Create some directories and files in the media dir
	os.Mkdir(filepath.Join(dirs.Media, "Some Movie"), 0755)
	os.Mkdir(filepath.Join(dirs.Media, "Artist - Album"), 0755)
	os.WriteFile(filepath.Join(dirs.Media, "random.txt"), []byte("hi"), 0644)

	names, err := List(dirs, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(names), names)
	}

	// Verify all entries are present (order depends on mtime, which is nearly
	// identical here, so just check membership)
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"Some Movie", "Artist - Album", "random.txt"} {
		if !found[want] {
			t.Errorf("expected %q in list, got %v", want, names)
		}
	}
}

func TestOrganizeArtist_SingleDisc(t *testing.T) {
	dirs := testDirs(t)

	albumDir := filepath.Join(dirs.Media, "Metallica - Master of Puppets")
	createFile(t, albumDir, "01 - Battery.flac", "audio-data-1")
	createFile(t, albumDir, "02 - Master of Puppets.flac", "audio-data-2")

	linked, err := OrganizeArtist(dirs, "Metallica")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	// Verify files exist in jellyfin music dir
	destDir := filepath.Join(dirs.JellyfinMusic, "Metallica", "Master of Puppets")
	for _, name := range []string{"01 - Battery.flac", "02 - Master of Puppets.flac"} {
		path := filepath.Join(destDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected %s to be a file, not a directory", path)
		}
	}

	// Verify hard link (same inode)
	srcInfo, _ := os.Stat(filepath.Join(albumDir, "01 - Battery.flac"))
	dstInfo, _ := os.Stat(filepath.Join(destDir, "01 - Battery.flac"))
	if !os.SameFile(srcInfo, dstInfo) {
		t.Error("expected source and destination to be hard links (same inode)")
	}
}

func TestOrganizeArtist_MultiDisc(t *testing.T) {
	dirs := testDirs(t)

	albumDir := filepath.Join(dirs.Media, "Pink Floyd - The Wall")
	createFile(t, filepath.Join(albumDir, "Disc 1"), "01 - In the Flesh.flac", "audio")
	createFile(t, filepath.Join(albumDir, "Disc 2"), "01 - Hey You.flac", "audio")

	linked, err := OrganizeArtist(dirs, "Pink Floyd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	destBase := filepath.Join(dirs.JellyfinMusic, "Pink Floyd", "The Wall")
	for _, check := range []string{
		filepath.Join(destBase, "Disc 1", "01 - In the Flesh.flac"),
		filepath.Join(destBase, "Disc 2", "01 - Hey You.flac"),
	} {
		if _, err := os.Stat(check); err != nil {
			t.Errorf("expected file %s to exist: %v", check, err)
		}
	}
}

func TestOrganizeArtist_MultipleAlbums(t *testing.T) {
	dirs := testDirs(t)

	createFile(t, filepath.Join(dirs.Media, "Radiohead - OK Computer"), "01 - Airbag.flac", "audio")
	createFile(t, filepath.Join(dirs.Media, "Radiohead - Kid A"), "01 - Everything in Its Right Place.flac", "audio")

	linked, err := OrganizeArtist(dirs, "Radiohead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	for _, path := range []string{
		filepath.Join(dirs.JellyfinMusic, "Radiohead", "OK Computer", "01 - Airbag.flac"),
		filepath.Join(dirs.JellyfinMusic, "Radiohead", "Kid A", "01 - Everything in Its Right Place.flac"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}
}

func TestOrganizeArtist_CaseInsensitive(t *testing.T) {
	dirs := testDirs(t)

	createFile(t, filepath.Join(dirs.Media, "TOOL - Lateralus"), "01 - The Grudge.flac", "audio")

	linked, err := OrganizeArtist(dirs, "tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 1 {
		t.Fatalf("expected 1 linked file, got %d: %v", len(linked), linked)
	}
}

func TestOrganizeArtist_SkipsExisting(t *testing.T) {
	dirs := testDirs(t)

	albumDir := filepath.Join(dirs.Media, "Deftones - White Pony")
	createFile(t, albumDir, "01 - Feiticeira.flac", "audio")

	// Run once
	_, err := OrganizeArtist(dirs, "Deftones")
	if err != nil {
		t.Fatalf("first run: unexpected error: %v", err)
	}

	// Run again — should not error, should report already exists
	linked, err := OrganizeArtist(dirs, "Deftones")
	if err != nil {
		t.Fatalf("second run: unexpected error: %v", err)
	}

	if len(linked) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(linked), linked)
	}
	if filepath.Base(linked[0]) != "01 - Feiticeira.flac (already exists)" {
		t.Errorf("expected 'already exists' note, got %q", linked[0])
	}
}

func TestOrganizeArtist_NoMatch(t *testing.T) {
	dirs := testDirs(t)

	_, err := OrganizeArtist(dirs, "Nonexistent Artist")
	if err == nil {
		t.Fatal("expected error for no matching directories, got nil")
	}
}

func TestFindVideoFiles(t *testing.T) {
	dirs := testDirs(t)
	showDir := filepath.Join(dirs.Media, "Breaking Bad")
	createFile(t, showDir, "Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv", "video1")
	createFile(t, showDir, "other.txt", "ignore")
	createFile(t, showDir, "episode.mp4", "video2")
	subDir := filepath.Join(showDir, "sub")
	createFile(t, subDir, "nested.mkv", "video3")

	videos, err := FindVideoFiles(showDir)
	if err != nil {
		t.Fatalf("FindVideoFiles: %v", err)
	}
	if len(videos) != 3 {
		t.Fatalf("expected 3 video files, got %d: %v", len(videos), videos)
	}
	names := make(map[string]bool)
	for _, p := range videos {
		names[filepath.Base(p)] = true
	}
	for _, want := range []string{"Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv", "episode.mp4", "nested.mkv"} {
		if !names[want] {
			t.Errorf("expected video %q in result", want)
		}
	}
}

func TestPublishTVSeason(t *testing.T) {
	dirs := testDirs(t)
	showDir := filepath.Join(dirs.Media, "Breaking Bad")
	createFile(t, showDir, "Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv", "pilot")
	createFile(t, showDir, "Breaking.Bad.S01E02.Cats.In.The.Bag.720p.mkv", "ep2")

	linked, err := PublishTVSeason(dirs, "Breaking Bad", "Breaking Bad (2008)")
	if err != nil {
		t.Fatalf("PublishTVSeason: %v", err)
	}
	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	destBase := filepath.Join(dirs.JellyfinTV, "Breaking Bad (2008)", "Season 1")
	for _, name := range []string{"Breaking Bad (2008) - S01E01.mkv", "Breaking Bad (2008) - S01E02.mkv"} {
		destPath := filepath.Join(destBase, name)
		info, err := os.Stat(destPath)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", destPath, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected %s to be a file, not a directory", destPath)
		}
	}

	// Verify hardlinks (same inode as source)
	src1 := filepath.Join(showDir, "Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv")
	dst1 := filepath.Join(destBase, "Breaking Bad (2008) - S01E01.mkv")
	srcInfo, _ := os.Stat(src1)
	dstInfo, _ := os.Stat(dst1)
	if !os.SameFile(srcInfo, dstInfo) {
		t.Error("expected source and destination S01E01 to be hard links (same inode)")
	}

	src2 := filepath.Join(showDir, "Breaking.Bad.S01E02.Cats.In.The.Bag.720p.mkv")
	dst2 := filepath.Join(destBase, "Breaking Bad (2008) - S01E02.mkv")
	srcInfo2, _ := os.Stat(src2)
	dstInfo2, _ := os.Stat(dst2)
	if !os.SameFile(srcInfo2, dstInfo2) {
		t.Error("expected source and destination S01E02 to be hard links (same inode)")
	}
}
