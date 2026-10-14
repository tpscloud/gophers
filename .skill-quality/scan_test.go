// Copyright 2026 John Deatherage (@coversheets). SPDX-License-Identifier: MIT
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func scannerArchive(t *testing.T, kind byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tarWriter := tar.NewWriter(gz)
	data := []byte("fixture, never executed")
	header := &tar.Header{Name: "gitleaks", Typeflag: kind, Mode: 0700}
	if kind == tar.TypeReg {
		header.Size = int64(len(data))
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if kind == tar.TypeReg {
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestScannerArchive(t *testing.T) {
	b := scannerArchive(t, tar.TypeReg)
	if _, err := extractScanner(b, "bad"); err == nil {
		t.Fatal("bad digest accepted")
	}
	if got, err := extractScanner(b, hashBytes(b)); err != nil || string(got) != "fixture, never executed" {
		t.Fatalf("%q %v", got, err)
	}
	b = scannerArchive(t, tar.TypeSymlink)
	if _, err := extractScanner(b, hashBytes(b)); err == nil {
		t.Fatal("symlink archive executable accepted")
	}
}

func TestScannerModesAndEvidence(t *testing.T) {
	cases := []struct {
		name    string
		codes   [2]int
		reports [2]string
		want    int
	}{
		{"clean", [2]int{0, 0}, [2]string{"[]", "[]"}, 0},
		{"history finding", [2]int{10, 0}, [2]string{`[{"Secret":"REDACTED"}]`, "[]"}, 1},
		{"file finding", [2]int{0, 10}, [2]string{"[]", `[{}]`}, 1},
		{"tool failure", [2]int{0, 2}, [2]string{"[]", "[]"}, 2},
		{"missing evidence", [2]int{0, 0}, [2]string{"", "[]"}, 2},
		{"malformed evidence", [2]int{0, 0}, [2]string{"{", "[]"}, 2},
		{"null evidence", [2]int{0, 0}, [2]string{"null", "[]"}, 2},
		{"contradictory clean", [2]int{0, 0}, [2]string{"[{}]", "[]"}, 2},
		{"contradictory findings", [2]int{10, 0}, [2]string{"[]", "[]"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := t.TempDir()
			calls := 0
			run := func(ctx context.Context, cwd string, args []string, out, diagnostic io.Writer) error {
				i := calls
				calls++
				if tc.reports[i] != "" {
					writeFixture(t, output, "gitleaks-"+args[1]+".json", tc.reports[i])
				}
				if tc.codes[i] == 0 {
					return nil
				}
				return exec.CommandContext(ctx, "sh", "-c", "exit "+strconv.Itoa(tc.codes[i])).Run()
			}
			modes, err := scannerResults(t.Context(), t.TempDir(), output, "fixture", run, io.Discard, io.Discard)
			if calls != 2 || len(modes) != 2 || exitCode(err) != tc.want {
				t.Fatalf("calls=%d modes=%+v err=%v", calls, modes, err)
			}
		})
	}
}

func TestScannerOutputBoundary(t *testing.T) {
	root := fixture(t)
	err := scanRepository(t.Context(), root, filepath.Join(root, "reports"), io.Discard, io.Discard)
	if err == nil || errors.Is(err, errFindings) {
		t.Fatal("accepted output inside repository")
	}
	output := t.TempDir()
	if err := scanRepository(t.Context(), root, output, io.Discard, io.Discard); err == nil {
		t.Fatal("accepted stale evidence directory")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if err := scanRepository(t.Context(), root, filepath.Join(link, "reports"), io.Discard, io.Discard); err == nil {
		t.Fatal("accepted symlink escape")
	}
}
