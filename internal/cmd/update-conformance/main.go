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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	plutusRevision      = "283ed666bac0bdd0548b6af90a0834f9b422f337"
	plutusArchiveSHA256 = "cf1dba1fa69152e3361f6cd97dfc39ee458429beec701edaf4d3c92e2f8ea520"
	maxArchiveBytes     = 128 << 20
	maxArchivePayload   = 512 << 20
	maxArchiveEntries   = 100_000
	maxExtractedBytes   = 512 << 20
	maxFixtureBytes     = 16 << 20
	maxFixtureFiles     = 20_000
)

var errArchiveTooLarge = errors.New("archive exceeds size limit")

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Minute}
	if err := refreshCorpus(ctx, client, "tests/conformance"); err != nil {
		fmt.Fprintf(os.Stderr, "update conformance corpus: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Updated tests/conformance from pinned Plutus source")
}

func refreshCorpus(
	ctx context.Context,
	client *http.Client,
	destination string,
) error {
	archive, err := os.CreateTemp("", "plutigo-conformance-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close archive file: %w", err)
	}

	archiveURL := "https://codeload.github.com/IntersectMBO/plutus/tar.gz/" +
		plutusRevision
	if err := downloadArchive(ctx, client, archiveURL, archivePath); err != nil {
		return err
	}
	return installCorpus(
		archivePath,
		destination,
		plutusArchiveSHA256,
		"plutus-"+plutusRevision,
	)
}

func downloadArchive(
	ctx context.Context,
	client *http.Client,
	archiveURL string,
	destination string,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return fmt.Errorf("create archive request: %w", err)
	}
	req.Header.Set("User-Agent", "plutigo-conformance-updater")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download archive: HTTP %s", resp.Status)
	}

	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	written, copyErr := io.Copy(out, io.LimitReader(resp.Body, maxArchiveBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("write archive: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close archive: %w", closeErr)
	}
	if written > maxArchiveBytes {
		return errArchiveTooLarge
	}
	return nil
}

func installCorpus(
	archivePath string,
	destination string,
	expectedDigest string,
	expectedRoot string,
) error {
	return installCorpusWithRename(
		archivePath,
		destination,
		expectedDigest,
		expectedRoot,
		os.Rename,
	)
}

func installCorpusWithRename(
	archivePath string,
	destination string,
	expectedDigest string,
	expectedRoot string,
	rename func(string, string) error,
) error {
	if err := verifyArchiveDigest(archivePath, expectedDigest); err != nil {
		return err
	}

	parent := filepath.Dir(destination)
	stagingRoot, err := os.MkdirTemp(parent, ".conformance-refresh-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	keepStaging := false
	defer func() {
		if !keepStaging {
			_ = os.RemoveAll(stagingRoot)
		}
	}()

	stagedCorpus := filepath.Join(stagingRoot, "new")
	if err := os.Mkdir(stagedCorpus, 0o755); err != nil {
		return fmt.Errorf("create staged corpus: %w", err)
	}
	if err := extractFixtures(archivePath, stagedCorpus, expectedRoot); err != nil {
		return err
	}
	if err := validateCorpus(stagedCorpus); err != nil {
		return err
	}

	if info, err := os.Lstat(destination); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("destination is not a directory")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect destination: %w", err)
	}

	backup := filepath.Join(stagingRoot, "previous")
	hadDestination := false
	if err := rename(destination, backup); err == nil {
		hadDestination = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stage existing corpus: %w", err)
	}
	if err := rename(stagedCorpus, destination); err != nil {
		if hadDestination {
			if restoreErr := rename(backup, destination); restoreErr != nil {
				keepStaging = true
				return fmt.Errorf(
					"install corpus: %w; restore previous corpus: %v; previous corpus preserved at %s",
					err,
					restoreErr,
					backup,
				)
			}
		}
		return fmt.Errorf("install corpus: %w", err)
	}
	return nil
}

func verifyArchiveDigest(archivePath, expectedDigest string) error {
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}
	if info.Size() > maxArchiveBytes {
		return errArchiveTooLarge
	}

	want, err := hex.DecodeString(expectedDigest)
	if err != nil || len(want) != sha256.Size {
		return errors.New("invalid configured archive digest")
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	if !bytes.Equal(h.Sum(nil), want) {
		return errors.New("archive SHA-256 does not match pinned digest")
	}
	return nil
}

func extractFixtures(archivePath, destination, expectedRoot string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open compressed archive: %w", err)
	}
	defer gz.Close()

	prefix := expectedRoot + "/plutus-conformance/test-cases/uplc/evaluation"
	reader := tar.NewReader(gz)
	var archiveEntries int
	var archivePayload int64
	var fileCount int
	var extractedBytes int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader &&
			header.Name == "pax_global_header" {
			continue
		}
		archiveEntries++
		if archiveEntries > maxArchiveEntries {
			return errors.New("archive exceeds entry limit")
		}
		if header.Size < 0 || header.Size > maxArchivePayload-archivePayload {
			return errors.New("archive exceeds payload limit")
		}
		archivePayload += header.Size

		archivePath, err := validateArchivePath(header.Name, expectedRoot)
		if err != nil {
			return err
		}
		if archivePath != prefix && !strings.HasPrefix(archivePath, prefix+"/") {
			continue
		}
		relative := strings.TrimPrefix(archivePath, prefix)
		relative = strings.TrimPrefix(relative, "/")
		if relative == "" {
			if header.Typeflag != tar.TypeDir {
				return errors.New("fixture root is not a directory")
			}
			continue
		}

		target := filepath.Join(destination, filepath.FromSlash(relative))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create fixture directory: %w", err)
			}
		case tar.TypeReg:
			if !allowedFixture(relative) {
				continue
			}
			if header.Mode&0o111 != 0 {
				return fmt.Errorf("fixture %q is executable", relative)
			}
			if header.Size < 0 || header.Size > maxFixtureBytes {
				return fmt.Errorf("fixture %q exceeds size limit", relative)
			}
			fileCount++
			extractedBytes += header.Size
			if fileCount > maxFixtureFiles || extractedBytes > maxExtractedBytes {
				return errors.New("fixture corpus exceeds extraction limits")
			}
			if err := writeFixture(reader, target, header.Size); err != nil {
				return fmt.Errorf("extract fixture %q: %w", relative, err)
			}
		default:
			return fmt.Errorf("fixture %q has unsupported archive type", relative)
		}
	}
	if fileCount == 0 {
		return errors.New("archive contains no allowed fixtures")
	}
	return nil
}

func validateArchivePath(name, expectedRoot string) (string, error) {
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("archive contains invalid path %q", name)
	}
	clean := path.Clean(trimmed)
	if clean != trimmed || path.IsAbs(clean) || clean == ".." ||
		strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive contains unsafe path %q", name)
	}
	if clean != expectedRoot && !strings.HasPrefix(clean, expectedRoot+"/") {
		return "", fmt.Errorf("archive path %q is outside pinned root", name)
	}
	return clean, nil
}

func allowedFixture(name string) bool {
	base := path.Base(name)
	if base == "README.md" {
		return true
	}
	for _, suffix := range []string{
		".budget.expected",
		".flat.expected",
		".uplc.expected",
		".flat",
		".uplc",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func writeFixture(reader io.Reader, target string, size int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	written, copyErr := io.CopyN(f, reader, size)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if written != size {
		return io.ErrUnexpectedEOF
	}
	return closeErr
}

func validateCorpus(root string) error {
	inputs := 0
	err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(name)
		if ext != ".uplc" && ext != ".flat" {
			return nil
		}
		inputs++
		if _, err := os.Stat(name + ".expected"); err != nil {
			return fmt.Errorf("fixture %q has no expected result", name)
		}
		budget := strings.TrimSuffix(name, ext) + ".budget.expected"
		if _, err := os.Stat(budget); err != nil {
			legacyBudget := name + ".budget.expected"
			if _, legacyErr := os.Stat(legacyBudget); legacyErr != nil {
				return fmt.Errorf("fixture %q has no expected budget", name)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("validate staged corpus: %w", err)
	}
	if inputs == 0 {
		return errors.New("staged corpus contains no test inputs")
	}
	return nil
}
