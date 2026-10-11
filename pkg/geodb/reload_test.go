package geodb

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const (
	testZips     = "../../internal/handler/testdata/zips.sqlite3"
	testGeonames = "../../internal/handler/testdata/geonames.sqlite3"
	atlantaID    = 4180439
)

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// rebuild writes a copy of the fixture with Atlanta renamed and moves it over
// path the way the weekly job does: a complete file, then an atomic rename.
func rebuild(t *testing.T, path, newName string) {
	t.Helper()
	tmp := path + ".tmp"
	copyFile(t, testGeonames, tmp)
	w, err := sql.Open("sqlite3", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Exec("UPDATE geoname SET name = ? WHERE geonameid = ?", newName, atlantaID); err != nil {
		t.Fatal(err)
	}
	w.Close()
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func TestReloadGeonames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geonames.sqlite3")
	copyFile(t, testGeonames, path)
	db, err := New(testZips, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if ok, err := db.ReloadGeonames(); ok || err != nil {
		t.Fatalf("unchanged file: ReloadGeonames() = %v, %v; want false, nil", ok, err)
	}
	if got := db.LookupGeoname(atlantaID).Name; got != "Atlanta, Georgia, USA" {
		t.Fatalf("before reload: %q", got)
	}

	// Look up continuously across the swap, so the race detector sees it.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if db.LookupGeoname(atlantaID) == nil {
					t.Error("lookup returned nil during reload")
					return
				}
				db.AutoComplete("atl", nil)
			}
		}()
	}

	rebuild(t, path, "Hotlanta")
	ok, err := db.ReloadGeonames()
	close(stop)
	wg.Wait()
	if !ok || err != nil {
		t.Fatalf("replaced file: ReloadGeonames() = %v, %v; want true, nil", ok, err)
	}
	if got := db.LookupGeoname(atlantaID).Name; got != "Hotlanta, Georgia, USA" {
		t.Fatalf("after reload: %q", got)
	}
	if ok, err := db.ReloadGeonames(); ok || err != nil {
		t.Fatalf("second check: ReloadGeonames() = %v, %v; want false, nil", ok, err)
	}

	// A broken replacement is refused and the current generation keeps serving.
	if err := os.WriteFile(path+".tmp", []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ReloadGeonames(); ok || err == nil {
		t.Fatalf("broken file: ReloadGeonames() = %v, %v; want false, error", ok, err)
	}
	if got := db.LookupGeoname(atlantaID).Name; got != "Hotlanta, Georgia, USA" {
		t.Fatalf("after failed reload: %q", got)
	}
}
