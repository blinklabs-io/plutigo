// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type archiveEntry struct {
	name     string
	typeflag byte
	mode     int64
	body     string
}

func TestInstallCorpus(t *testing.T) {
	root := "plutus-test-revision"
	prefix := root + "/plutus-conformance/test-cases/uplc/evaluation/case"
	archive, digest := writeTestArchive(t, []archiveEntry{
		{name: prefix + "/sample.uplc", body: "(program 1.1.0 (con unit ()))"},
		{name: prefix + "/sample.uplc.expected", body: "(con unit ())"},
		{name: prefix + "/sample.budget.expected", body: "(100, 100)"},
		{name: prefix + "/README.md", body: "fixture notes"},
		{name: prefix + "/ren.sh", mode: 0o755, body: "#!/bin/sh"},
	})
	destination := existingCorpus(t)

	if err := installCorpus(archive, destination, digest, root); err != nil {
		t.Fatalf("installCorpus() error = %v", err)
	}
	assertFileContents(t, filepath.Join(destination, "case", "sample.uplc"), "(program 1.1.0 (con unit ()))")
	if _, err := os.Stat(filepath.Join(destination, "case", "ren.sh")); !os.IsNotExist(err) {
		t.Fatalf("non-fixture file was installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("previous corpus was not replaced: %v", err)
	}
}

func TestInstallCorpusRejectsUnsafeArchivesBeforeReplacement(t *testing.T) {
	root := "plutus-test-revision"
	prefix := root + "/plutus-conformance/test-cases/uplc/evaluation/case"
	valid := []archiveEntry{
		{name: prefix + "/sample.uplc", body: "program"},
		{name: prefix + "/sample.uplc.expected", body: "result"},
		{name: prefix + "/sample.budget.expected", body: "budget"},
	}
	tests := []struct {
		name    string
		entries []archiveEntry
	}{
		{
			name: "wrong archive root",
			entries: []archiveEntry{
				{name: "plutus-other/file", body: "data"},
			},
		},
		{
			name: "path traversal",
			entries: appendCopy(valid, archiveEntry{
				name: prefix + "/../escape.uplc",
				body: "data",
			}),
		},
		{
			name: "symlink",
			entries: appendCopy(valid, archiveEntry{
				name:     prefix + "/link.uplc",
				typeflag: tar.TypeSymlink,
			}),
		},
		{
			name: "executable fixture",
			entries: []archiveEntry{
				{name: prefix + "/sample.uplc", mode: 0o755, body: "program"},
				{name: prefix + "/sample.uplc.expected", body: "result"},
				{name: prefix + "/sample.budget.expected", body: "budget"},
			},
		},
		{
			name: "unsupported file type",
			entries: appendCopy(valid, archiveEntry{
				name:     prefix + "/pipe.uplc",
				typeflag: tar.TypeFifo,
			}),
		},
		{
			name: "incomplete fixture",
			entries: []archiveEntry{
				{name: prefix + "/sample.uplc", body: "program"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive, digest := writeTestArchive(t, test.entries)
			destination := existingCorpus(t)

			if err := installCorpus(archive, destination, digest, root); err == nil {
				t.Fatal("installCorpus() succeeded for unsafe archive")
			}
			assertFileContents(t, filepath.Join(destination, "old.txt"), "old corpus")
		})
	}
}

func TestInstallCorpusRejectsDigestMismatchBeforeReplacement(t *testing.T) {
	root := "plutus-test-revision"
	prefix := root + "/plutus-conformance/test-cases/uplc/evaluation/case"
	archive, _ := writeTestArchive(t, []archiveEntry{
		{name: prefix + "/sample.uplc", body: "program"},
		{name: prefix + "/sample.uplc.expected", body: "result"},
		{name: prefix + "/sample.budget.expected", body: "budget"},
	})
	destination := existingCorpus(t)

	err := installCorpus(archive, destination, strings.Repeat("0", 64), root)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("installCorpus() error = %v, want digest mismatch", err)
	}
	assertFileContents(t, filepath.Join(destination, "old.txt"), "old corpus")
}

func TestFixtureTargetValidatesLocalPath(t *testing.T) {
	destination := t.TempDir()

	got, err := fixtureTarget(destination, "case/sample.uplc")
	if err != nil {
		t.Fatalf("fixtureTarget() error = %v", err)
	}
	if want := filepath.Join(destination, "case", "sample.uplc"); got != want {
		t.Fatalf("fixtureTarget() = %q, want %q", got, want)
	}

	got, err = fixtureTarget(destination, "../escape.uplc")
	if err == nil || got != "" {
		t.Fatalf("fixtureTarget() = (%q, %v), want rejected path", got, err)
	}
}

func TestInstallCorpusPreservesBackupWhenRestoreFails(t *testing.T) {
	root := "plutus-test-revision"
	prefix := root + "/plutus-conformance/test-cases/uplc/evaluation/case"
	archive, digest := writeTestArchive(t, []archiveEntry{
		{name: prefix + "/sample.uplc", body: "program"},
		{name: prefix + "/sample.uplc.expected", body: "result"},
		{name: prefix + "/sample.budget.expected", body: "budget"},
	})
	destination := existingCorpus(t)
	renameCalls := 0
	rename := func(oldPath, newPath string) error {
		renameCalls++
		if renameCalls > 1 {
			return os.ErrPermission
		}
		return os.Rename(oldPath, newPath)
	}

	err := installCorpusWithRename(archive, destination, digest, root, rename)
	if err == nil || !strings.Contains(err.Error(), "previous corpus preserved at") {
		t.Fatalf("installCorpusWithRename() error = %v, want preserved backup path", err)
	}
	if renameCalls != 3 {
		t.Fatalf("rename called %d times, want install and restore failures after backup", renameCalls)
	}
	backup := strings.SplitN(err.Error(), "; previous corpus preserved at ", 2)[1]
	assertFileContents(t, filepath.Join(backup, "old.txt"), "old corpus")
}

func appendCopy(entries []archiveEntry, extra archiveEntry) []archiveEntry {
	result := append([]archiveEntry(nil), entries...)
	return append(result, extra)
}

func existingCorpus(t *testing.T) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "tests", "conformance")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "old.txt"), []byte("old corpus"), 0o644); err != nil {
		t.Fatal(err)
	}
	return destination
}

func writeTestArchive(t *testing.T, entries []archiveEntry) (string, string) {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "fixtures.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		mode := entry.mode
		if mode == 0 {
			mode = 0o644
		}
		header := &tar.Header{
			Name:     entry.name,
			Mode:     mode,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if entry.body != "" {
			if _, err := tw.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	return archivePath, hex.EncodeToString(digest[:])
}

func assertFileContents(t *testing.T, name, want string) {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %q: %v", name, err)
	}
	if string(contents) != want {
		t.Fatalf("%q contents = %q, want %q", name, contents, want)
	}
}
