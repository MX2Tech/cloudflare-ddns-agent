package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFile_AppendsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")

	l, err := Open(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	l.Write([]byte("first\n"))
	l.Close()

	l, err = Open(path, 1024)
	if err != nil {
		t.Fatal(err)
	}
	l.Write([]byte("second\n"))
	l.Close()

	got, _ := os.ReadFile(path)
	if string(got) != "first\nsecond\n" {
		t.Errorf("log was not appended: %q", got)
	}
}

func TestFile_RotatesAndKeepsOneOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")

	l, err := Open(path, 20)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	for _, line := range []string{"aaaaaaaaa\n", "bbbbbbbbb\n", "ccccccccc\n", "ddddddddd\n", "eeeeeeeee\n"} {
		if _, err := l.Write([]byte(line)); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	current, _ := os.ReadFile(path)
	old, _ := os.ReadFile(path + ".1")
	if string(current) != "eeeeeeeee\n" {
		t.Errorf("current log: %q", current)
	}
	if string(old) != "ccccccccc\nddddddddd\n" {
		t.Errorf("rotated log: %q", old)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected exactly 2 files, got %s", strings.Join(names, ", "))
	}
}

func TestFile_OversizedSingleWriteStillLands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	l, err := Open(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	if _, err := l.Write([]byte("longer than the cap\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "longer than the cap\n" {
		t.Errorf("oversized line lost: %q", got)
	}
}
