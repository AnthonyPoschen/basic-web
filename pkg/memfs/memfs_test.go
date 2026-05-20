package memfs

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestCreateMinifiedFSPreservesNestedDirectories(t *testing.T) {
	mem := CreateMinifiedFS(fstest.MapFS{
		"elements/pages/page-home.html": {
			Data: []byte(`<template id="page-home"></template>`),
		},
		"elements/x-counter.html": {
			Data: []byte(`<template id="x-counter"></template>`),
		},
	})

	var paths []string
	err := fs.WalkDir(mem, "elements", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk nested elements: %v", err)
	}

	expected := []string{
		"elements",
		"elements/pages",
		"elements/pages/page-home.html",
		"elements/x-counter.html",
	}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatalf("walked paths mismatch\n got: %#v\nwant: %#v", paths, expected)
	}

	entries, err := fs.ReadDir(mem, "elements/pages")
	if err != nil {
		t.Fatalf("read nested directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "page-home.html" {
		t.Fatalf("nested directory entries = %#v, want page-home.html", entries)
	}
}
