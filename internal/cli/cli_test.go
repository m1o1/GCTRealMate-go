package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "example.asm")
	exe := filepath.Join(dir, "gctrm.exe")
	if e := os.WriteFile(input, []byte("Example\nop nop @ $80001000\n"), 0600); e != nil {
		t.Fatal(e)
	}
	var out, err bytes.Buffer
	if code := runWithFixes(context.Background(), []string{"-q", "-g", "-l", input}, exe, &out, &err); code != 0 {
		t.Fatalf("%d %s", code, &err)
	}
	for _, name := range []string{"example.GCT", "example_codeset.txt", "example_log.txt"} {
		if _, e := os.Stat(filepath.Join(dir, name)); e != nil {
			t.Fatal(e)
		}
	}
	text, e := os.ReadFile(filepath.Join(dir, "example_codeset.txt"))
	if e != nil || !strings.Contains(string(text), "* 04001000 60000000") {
		t.Fatal(string(text), e)
	}
}
func TestFailurePreservesOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bad.asm")
	output := filepath.Join(dir, "bad.GCT")
	os.WriteFile(input, []byte("Example\nHOOK @ $80001000\n{\nb absent\n}"), 0600)
	os.WriteFile(output, []byte("keep me"), 0600)
	var out, err bytes.Buffer
	if code := runWithFixes(context.Background(), []string{input}, "", &out, &err); code != 1 {
		t.Fatal(code)
	}
	data, _ := os.ReadFile(output)
	if string(data) != "keep me" {
		t.Fatal("existing output changed")
	}
}
func TestSettingsAndScopedOptions(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "gctrm.exe")
	os.WriteFile(strings.TrimSuffix(exe, ".exe")+".ini", []byte("first.asm : -a -b 0x80500000 -t\nfirst.asm.extra : -l\n"), 0600)
	jobs, _, e := plan([]string{"-l", "first.asm", "-a:0", "second.asm"}, exe)
	if e != nil {
		t.Fatal(e)
	}
	if len(jobs) != 2 || !jobs[0].flags.inline || jobs[0].flags.base == nil || *jobs[0].flags.base != 0x80500000 || !jobs[0].flags.text || !jobs[0].flags.log {
		t.Fatal(jobs)
	}
	if jobs[1].flags.inline || jobs[1].flags.base != nil || !jobs[1].flags.text || !jobs[1].flags.log {
		t.Fatal(jobs[1])
	}
	jobs, _, e = plan([]string{"-i", "first.asm"}, exe)
	if e != nil || jobs[0].flags.text {
		t.Fatal(jobs, e)
	}
	jobs, _, e = plan([]string{"first.asm.extra2"}, exe)
	if e != nil || jobs[0].flags.text {
		t.Fatal(jobs, e)
	}
}
func TestCLIValidation(t *testing.T) {
	for _, args := range [][]string{{"-z", "file.asm"}, {"-t:2", "file.asm"}, {"-b"}, {"-b:xyz", "file.asm"}, {"-b:80000001", "file.asm"}, {"-q"}, {"file.asm", "-l"}, {"-o", "same.asm", "same.asm"}, {"-o", "out.GCT", "a.asm", "b.asm"}, {"a.asm", "a.txt"}, {"a.asm", "a.GCT"}} {
		if _, _, e := plan(args, ""); e == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
func TestHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"--version"}} {
		var out, err bytes.Buffer
		if code := runWithFixes(context.Background(), args, "", &out, &err); code != 0 || out.Len() == 0 {
			t.Fatal(code, out.String(), err.String())
		}
	}
}
func TestMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	var files []string
	for _, name := range []string{"first.asm", "second.asm"} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte("Example\nop li r3,1 @ $80001000"), 0600)
		files = append(files, path)
	}
	var out, err bytes.Buffer
	if code := runWithFixes(context.Background(), files, "", &out, &err); code != 0 {
		t.Fatal(code, &err)
	}
}
