package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunReportsErrorOnce(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"list", "extra"}, &out, &errb); code != exitFailure {
		t.Fatalf("code=%d", code)
	}
	if out.Len() != 0 || strings.Count(errb.String(), "catalog:") != 1 {
		t.Fatalf("out=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunWritesPayloadToStdout(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"list", "--tag", "go", "--json"}, &out, &errb); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if out.String() != "[\"cobra\",\"pflag\"]\n" || errb.Len() != 0 {
		t.Fatalf("out=%q stderr=%q", out.String(), errb.String())
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--version"}, &out, &errb); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
	if out.String() != "catalog version dev\n" {
		t.Fatalf("out=%q", out.String())
	}
}
