package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func noTags(context.Context) ([]string, error) { return nil, nil }

func TestListJSON(t *testing.T) {
	var out, diagnostics bytes.Buffer
	root := NewRoot(func(ctx context.Context, tag string) ([]string, error) {
		if ctx != t.Context() || tag != "go" {
			t.Fatalf("context or tag not forwarded: %q", tag)
		}
		return []string{"example"}, nil
	}, noTags)
	root.SetArgs([]string{"list", "--tag", "go", "--json"})
	root.SetOut(&out)
	root.SetErr(&diagnostics)
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal(out.Bytes(), &names); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"example"}) || diagnostics.Len() != 0 {
		t.Fatalf("payload=%q diagnostics=%q", out.String(), diagnostics.String())
	}
}

func TestInvalidInputSkipsService(t *testing.T) {
	for _, args := range [][]string{
		{"list", "extra"},
		{"list", "--unknown"},
	} {
		var out, diagnostics bytes.Buffer
		root := NewRoot(func(context.Context, string) ([]string, error) {
			t.Fatalf("%v: service called", args)
			return nil, nil
		}, noTags)
		root.SetArgs(args)
		root.SetOut(&out)
		root.SetErr(&diagnostics)
		if err := root.ExecuteContext(t.Context()); err == nil {
			t.Fatalf("%v: expected an error", args)
		}
		// SilenceErrors and SilenceUsage leave reporting to the caller.
		if out.Len() != 0 || diagnostics.Len() != 0 {
			t.Fatalf("%v: out=%q diagnostics=%q", args, out.String(), diagnostics.String())
		}
	}
}

func TestServiceErrorIsReturned(t *testing.T) {
	boom := errors.New("backend unavailable")
	root := NewRoot(func(context.Context, string) ([]string, error) { return nil, boom }, noTags)
	root.SetArgs([]string{"list"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.ExecuteContext(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestCancellationReachesService(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	root := NewRoot(func(ctx context.Context, _ string) ([]string, error) {
		return nil, ctx.Err()
	}, noTags)
	root.SetArgs([]string{"list"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

type failingWriter struct{}

var errClosed = errors.New("stdout closed")

func (failingWriter) Write([]byte) (int, error) { return 0, errClosed }

func TestOutputFailureIsReturned(t *testing.T) {
	root := NewRoot(func(context.Context, string) ([]string, error) { return []string{"a"}, nil }, noTags)
	root.SetArgs([]string{"list"})
	root.SetOut(failingWriter{})
	root.SetErr(new(bytes.Buffer))
	if err := root.ExecuteContext(t.Context()); !errors.Is(err, errClosed) {
		t.Fatalf("err=%v", err)
	}
}

func TestEmptyResultIsJSONArray(t *testing.T) {
	var out bytes.Buffer
	root := NewRoot(func(context.Context, string) ([]string, error) { return nil, nil }, noTags)
	root.SetArgs([]string{"list", "--json"})
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[]\n" {
		t.Fatalf("payload=%q", out.String())
	}
}

func TestTagCompletion(t *testing.T) {
	var out bytes.Buffer
	root := NewRoot(func(context.Context, string) ([]string, error) {
		t.Fatal("completion must not run the command")
		return nil, nil
	}, func(context.Context) ([]string, error) { return []string{"go", "git", "rust"}, nil })
	root.SetArgs([]string{"__complete", "list", "--tag", "g"})
	root.SetOut(&out)
	root.SetErr(new(bytes.Buffer))
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Candidates, then ":<directive>"; 4 is ShellCompDirectiveNoFileComp.
	if out.String() != "go\ngit\n:4\n" {
		t.Fatalf("completion=%q", out.String())
	}
}
