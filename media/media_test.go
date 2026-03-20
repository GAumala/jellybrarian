package media

import (
	"os"
	"path/filepath"
	"testing"
)

// testEnv is a temp tree with media + three jellyfin library roots.
func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	root := t.TempDir()
	e := testEnv{
		Media:  filepath.Join(root, "media"),
		Music:  filepath.Join(root, "jellyfin", "music"),
		Movies: filepath.Join(root, "jellyfin", "movies"),
		TV:     filepath.Join(root, "jellyfin", "tv"),
	}
	for _, d := range []string{e.Media, e.Music, e.Movies, e.TV} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}
	return e
}

type testEnv struct {
	Media  string
	Music  string
	Movies string
	TV     string
}

func (e testEnv) mgrMusic() MediaManager {
	return MediaManager{MediaDir: e.Media, LibraryDir: e.Music}
}

func (e testEnv) mgrMovies() MediaManager {
	return MediaManager{MediaDir: e.Media, LibraryDir: e.Movies}
}

func (e testEnv) mgrTV() MediaManager {
	return MediaManager{MediaDir: e.Media, LibraryDir: e.TV}
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

func TestListMedia_Empty(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	names, err := mgr.ListMedia(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty list, got %v", names)
	}
}

func TestListMedia_ReturnsEntries(t *testing.T) {
	env := newTestEnv(t)

	os.Mkdir(filepath.Join(env.Media, "Some Movie"), 0755)
	os.Mkdir(filepath.Join(env.Media, "Artist - Album"), 0755)
	os.WriteFile(filepath.Join(env.Media, "random.txt"), []byte("hi"), 0644)

	names, err := env.mgrMusic().ListMedia(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(names), names)
	}

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
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	albumDir := filepath.Join(env.Media, "Metallica - Master of Puppets")
	createFile(t, albumDir, "01 - Battery.flac", "audio-data-1")
	createFile(t, albumDir, "02 - Master of Puppets.flac", "audio-data-2")

	linked, err := mgr.OrganizeArtist("Metallica")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	destDir := filepath.Join(env.Music, "Metallica", "Master of Puppets")
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

	srcInfo, _ := os.Stat(filepath.Join(albumDir, "01 - Battery.flac"))
	dstInfo, _ := os.Stat(filepath.Join(destDir, "01 - Battery.flac"))
	if !os.SameFile(srcInfo, dstInfo) {
		t.Error("expected source and destination to be hard links (same inode)")
	}
}

func TestOrganizeArtist_MultiDisc(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	albumDir := filepath.Join(env.Media, "Pink Floyd - The Wall")
	createFile(t, filepath.Join(albumDir, "Disc 1"), "01 - In the Flesh.flac", "audio")
	createFile(t, filepath.Join(albumDir, "Disc 2"), "01 - Hey You.flac", "audio")

	linked, err := mgr.OrganizeArtist("Pink Floyd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	destBase := filepath.Join(env.Music, "Pink Floyd", "The Wall")
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
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	createFile(t, filepath.Join(env.Media, "Radiohead - OK Computer"), "01 - Airbag.flac", "audio")
	createFile(t, filepath.Join(env.Media, "Radiohead - Kid A"), "01 - Everything in Its Right Place.flac", "audio")

	linked, err := mgr.OrganizeArtist("Radiohead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	for _, path := range []string{
		filepath.Join(env.Music, "Radiohead", "OK Computer", "01 - Airbag.flac"),
		filepath.Join(env.Music, "Radiohead", "Kid A", "01 - Everything in Its Right Place.flac"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
		}
	}
}

func TestOrganizeArtist_CaseInsensitive(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	createFile(t, filepath.Join(env.Media, "TOOL - Lateralus"), "01 - The Grudge.flac", "audio")

	linked, err := mgr.OrganizeArtist("tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(linked) != 1 {
		t.Fatalf("expected 1 linked file, got %d: %v", len(linked), linked)
	}
}

func TestOrganizeArtist_SkipsExisting(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	albumDir := filepath.Join(env.Media, "Deftones - White Pony")
	createFile(t, albumDir, "01 - Feiticeira.flac", "audio")

	_, err := mgr.OrganizeArtist("Deftones")
	if err != nil {
		t.Fatalf("first run: unexpected error: %v", err)
	}

	linked, err := mgr.OrganizeArtist("Deftones")
	if err != nil {
		t.Fatalf("second run: unexpected error: %v", err)
	}

	if len(linked) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(linked), linked)
	}
	if filepath.Base(linked[0]) != "01 - Feiticeira.flac" {
		t.Errorf("expected destination path, got %q", linked[0])
	}
}

func TestOrganizeArtist_NoMatch(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMusic()

	_, err := mgr.OrganizeArtist("Nonexistent Artist")
	if err == nil {
		t.Fatal("expected error for no matching directories, got nil")
	}
}

func TestListLibraryTitles_TV(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrTV()

	names, err := mgr.ListLibraryTitles("")
	if err != nil {
		t.Fatalf("ListLibraryTitles: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected no titles, got %v", names)
	}

	os.Mkdir(filepath.Join(env.TV, "Breaking Bad (2008)"), 0755)
	os.Mkdir(filepath.Join(env.TV, "Succession"), 0755)
	os.Mkdir(filepath.Join(env.TV, "ONE PIECE (2023)"), 0755)

	names, err = mgr.ListLibraryTitles("")
	if err != nil {
		t.Fatalf("ListLibraryTitles: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 titles, got %d: %v", len(names), names)
	}
	want := []string{"Breaking Bad (2008)", "ONE PIECE (2023)", "Succession"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q (order should be sorted)", i, names[i], w)
		}
	}
}

func TestListLibraryTitles_Movies(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMovies()

	names, err := mgr.ListLibraryTitles("")
	if err != nil {
		t.Fatalf("ListLibraryTitles: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected no titles, got %v", names)
	}

	os.Mkdir(filepath.Join(env.Movies, "Inception (2010)"), 0755)
	os.Mkdir(filepath.Join(env.Movies, "The Lord of the Rings (2001)"), 0755)

	names, err = mgr.ListLibraryTitles("")
	if err != nil {
		t.Fatalf("ListLibraryTitles: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 titles, got %d: %v", len(names), names)
	}
}

func TestFilterTitlesByQuery(t *testing.T) {
	titles := []string{"Silicon Valley (2014)", "Succession", "ONE PIECE (2023)"}

	got := filterTitlesByQuery(titles, "")
	if len(got) != 3 {
		t.Fatalf("q empty: got %v", got)
	}

	got = filterTitlesByQuery(titles, "one")
	if len(got) != 1 || got[0] != "ONE PIECE (2023)" {
		t.Errorf("q=one: got %v", got)
	}

	got = filterTitlesByQuery(titles, "piece")
	if len(got) != 1 || got[0] != "ONE PIECE (2023)" {
		t.Errorf("q=piece: got %v", got)
	}

	got = filterTitlesByQuery(titles, "one piece")
	if len(got) != 1 || got[0] != "ONE PIECE (2023)" {
		t.Errorf("q=one piece: got %v", got)
	}

	got = filterTitlesByQuery(titles, "SUCCESSION")
	if len(got) != 1 || got[0] != "Succession" {
		t.Errorf("q=SUCCESSION: got %v", got)
	}

	got = filterTitlesByQuery([]string{"Café", "Office"}, "cafe")
	if len(got) != 1 || got[0] != "Café" {
		t.Errorf("accent: got %v", got)
	}
}

func TestFindVideoFiles(t *testing.T) {
	env := newTestEnv(t)
	showDir := filepath.Join(env.Media, "Breaking Bad")
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

func TestFindVideoFiles_SingleFile(t *testing.T) {
	env := newTestEnv(t)
	singleMkv := filepath.Join(env.Media, "movie.mkv")
	createFile(t, env.Media, "movie.mkv", "content")

	videos, err := FindVideoFiles(singleMkv)
	if err != nil {
		t.Fatalf("FindVideoFiles: %v", err)
	}
	if len(videos) != 1 || videos[0] != singleMkv {
		t.Fatalf("expected single path %q, got %v", singleMkv, videos)
	}

	txtPath := filepath.Join(env.Media, "readme.txt")
	createFile(t, env.Media, "readme.txt", "text")
	videos2, err := FindVideoFiles(txtPath)
	if err != nil {
		t.Fatalf("FindVideoFiles(txt): %v", err)
	}
	if len(videos2) != 0 {
		t.Fatalf("expected no videos for .txt file, got %v", videos2)
	}
}

func TestAddTVSeason(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrTV()

	showDir := filepath.Join(env.Media, "Breaking Bad")
	createFile(t, showDir, "Breaking.Bad.S01E01.Pilot.720p.WEB-DL.x264-GROUP.mkv", "pilot")
	createFile(t, showDir, "Breaking.Bad.S01E02.Cats.In.The.Bag.720p.mkv", "ep2")

	linked, err := mgr.AddTVSeason("Breaking Bad", "Breaking Bad (2008)")
	if err != nil {
		t.Fatalf("AddTVSeason: %v", err)
	}
	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	destBase := filepath.Join(env.TV, "Breaking Bad (2008)", "Season 1")
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

func TestAddMovie_SingleFile(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMovies()

	createFile(t, env.Media, "Inception.2010.1080p.mkv", "movie-content")

	linked, err := mgr.AddMovie("Inception.2010.1080p.mkv", "Inception (2010)")
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	if len(linked) != 1 {
		t.Fatalf("expected 1 linked file, got %d: %v", len(linked), linked)
	}

	destPath := filepath.Join(env.Movies, "Inception (2010)", "Inception (2010).mkv")
	if linked[0] != destPath {
		t.Errorf("expected %q, got %q", destPath, linked[0])
	}
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("destination file missing: %v", err)
	}
	if info.IsDir() {
		t.Error("expected destination to be a file")
	}
	srcPath := filepath.Join(env.Media, "Inception.2010.1080p.mkv")
	srcInfo, _ := os.Stat(srcPath)
	if !os.SameFile(srcInfo, info) {
		t.Error("expected source and destination to be hard links (same inode)")
	}
}

func TestAddMovie_MultipleParts(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMovies()

	movieDir := filepath.Join(env.Media, "Lord of the Rings")
	createFile(t, movieDir, "part1.mkv", "part1")
	createFile(t, movieDir, "part2.mkv", "part2")

	linked, err := mgr.AddMovie("Lord of the Rings", "The Lord of the Rings (2001)")
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	if len(linked) != 2 {
		t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
	}

	base := filepath.Join(env.Movies, "The Lord of the Rings (2001)")
	wantPaths := []string{
		filepath.Join(base, "The Lord of the Rings (2001)-part-1.mkv"),
		filepath.Join(base, "The Lord of the Rings (2001)-part-2.mkv"),
	}
	for _, destPath := range wantPaths {
		var found bool
		for _, p := range linked {
			if p == destPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected linked to contain %q, got %v", destPath, linked)
		}
		if _, err := os.Stat(destPath); err != nil {
			t.Errorf("file %s missing: %v", destPath, err)
		}
	}
	src1 := filepath.Join(movieDir, "part1.mkv")
	dst1 := filepath.Join(base, "The Lord of the Rings (2001)-part-1.mkv")
	if !os.SameFile(mustStat(t, src1), mustStat(t, dst1)) {
		t.Error("part1: expected hard link")
	}
}

func TestAddMovie_AudioAndSubtitles(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMovies()

	movieDir := filepath.Join(env.Media, "Dune")
	createFile(t, movieDir, "Dune.2021.1080p.mkv", "video")
	createFile(t, movieDir, "Dune.2021.es.aac", "spanish-audio")
	createFile(t, movieDir, "Dune.2021.en.srt", "english-subs")

	linked, err := mgr.AddMovie("Dune", "Dune (2021)")
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	if len(linked) != 3 {
		t.Fatalf("expected 3 linked files (video + audio + sub), got %d: %v", len(linked), linked)
	}

	base := filepath.Join(env.Movies, "Dune (2021)")
	wantNames := map[string]bool{
		"Dune (2021).mkv":    true,
		"Dune (2021).es.aac": true,
		"Dune (2021).en.srt": true,
	}
	for _, path := range linked {
		name := filepath.Base(path)
		if !wantNames[name] {
			t.Errorf("unexpected linked file: %s", path)
		}
	}
	srcAac := filepath.Join(movieDir, "Dune.2021.es.aac")
	dstAac := filepath.Join(base, "Dune (2021).es.aac")
	if !os.SameFile(mustStat(t, srcAac), mustStat(t, dstAac)) {
		t.Error("expected .es.aac to be hard linked")
	}
	srcSrt := filepath.Join(movieDir, "Dune.2021.en.srt")
	dstSrt := filepath.Join(base, "Dune (2021).en.srt")
	if !os.SameFile(mustStat(t, srcSrt), mustStat(t, dstSrt)) {
		t.Error("expected .en.srt to be hard linked")
	}
}

func TestAddMovie_ThreeLetterLangAndPlainSrt(t *testing.T) {
	env := newTestEnv(t)
	mgr := env.mgrMovies()

	movieDir := filepath.Join(env.Media, "Alien")
	createFile(t, movieDir, "Alien.1979.mkv", "video")
	createFile(t, movieDir, "Alien.1979.eng.srt", "english-subs")
	createFile(t, movieDir, "Alien.1979.srt", "no-lang-subs")
	createFile(t, movieDir, "Alien.1979.spa.aac", "spanish-audio")

	linked, err := mgr.AddMovie("Alien", "Alien (1979)")
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	if len(linked) != 4 {
		t.Fatalf("expected 4 linked files, got %d: %v", len(linked), linked)
	}

	wantNames := map[string]bool{
		"Alien (1979).mkv":    true,
		"Alien (1979).en.srt": true, // eng -> en
		"Alien (1979).srt":    true, // plain .srt, no lang
		"Alien (1979).es.aac": true, // spa -> es
	}
	for _, path := range linked {
		name := filepath.Base(path)
		if !wantNames[name] {
			t.Errorf("unexpected linked file: %s", path)
		}
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	return info
}
