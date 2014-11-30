// Licensed under the Apache License, Version 2.0; See LICENSE.APACHE

package symlink

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

type dirOrLink struct {
	path   string
	target string
}

func makeFs(tmpdir string, fs []dirOrLink) error {
	for _, s := range fs {
		s.path = filepath.Join(tmpdir, s.path)
		if s.target == "" {
			os.MkdirAll(s.path, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
			return err
		}
		if err := os.Symlink(s.target, s.path); err != nil && !os.IsExist(err) {
			return err
		}
	}
	return nil
}

func testSymlink(tmpdir, path, expected, scope string) error {
	rewrite, err := FollowSymlinkInScope(filepath.Join(tmpdir, path), filepath.Join(tmpdir, scope))
	if err != nil {
		return err
	}
	expected, err = filepath.Abs(filepath.Join(tmpdir, expected))
	if err != nil {
		return err
	}
	if expected != rewrite {
		return fmt.Errorf("Expected %q got %q", expected, rewrite)
	}
	return nil
}

func TestFollowSymlinkNormal(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkNormal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/a/d", target: "/b"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/a/d/c/data", "testdata/b/c/data", "testdata"); err != nil {
		t.Fatal(err)
	}
}

func TestFollowSymlinkRelativePath(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkRelativePath")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/i", target: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/i", "testdata/fs/a", "testdata"); err != nil {
		t.Fatal(err)
	}
}

func TestFollowSymlinkUnderLinkedDir(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkUnderLinkedDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := makeFs(tmpdir, []dirOrLink{
		{path: "linkdir", target: "realdir"},
		{path: "linkdir/foo/bar"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "linkdir/foo/bar", "linkdir/foo/bar", "linkdir/foo"); err != nil {
		t.Fatal(err)
	}
}

func TestFollowSymlinkRandomString(t *testing.T) {
	if _, err := FollowSymlinkInScope("toto", "testdata"); err == nil {
		t.Fatal("Random string should fail but didn't")
	}
}

func TestFollowSymlinkLastLink(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkLastLink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/a/d", target: "/b"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/a/d", "testdata/b", "testdata"); err != nil {
		t.Fatal(err)
	}
}

func TestFollowSymlinkRelativeLink(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkRelativeLink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/a/e", target: "../b"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/a/e/c/data", "testdata/fs/b/c/data", "testdata"); err != nil {
		t.Fatal(err)
	}
}

func TestFollowSymlinkRelativeLinkScope(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "TestFollowSymlinkRelativeLinkScope")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)

	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/a/f", target: "../../../../test"}}); err != nil {
		t.Fatal(err)
	}
	// avoid letting symlink f lead us out of the "testdata" scope
	// we don't normalize because symlink f is in scope and there is no
	// information leak
	if err := testSymlink(tmpdir, "testdata/fs/a/f", "testdata/test", "testdata"); err != nil {
		t.Fatal(err)
	}
	// avoid letting symlink f lead us out of the "testdata/fs" scope
	// we don't normalize because symlink f is in scope and there is no
	// information leak
	if err := testSymlink(tmpdir, "testdata/fs/a/f", "testdata/fs/test", "testdata/fs"); err != nil {
		t.Fatal(err)
	}

	// avoid letting symlink g (pointed at by symlink h) take out of scope
	// TODO: we should probably normalize to scope here because ../[....]/root
	// is out of scope and we leak information
	if err := makeFs(tmpdir, []dirOrLink{
		{path: "testdata/fs/b/h", target: "../g"},
		{path: "testdata/fs/g", target: "../../../../../../../../../../../../root"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/b/h", "testdata/root", "testdata"); err != nil {
		t.Fatal(err)
	}

	// avoid letting allowing symlink e lead us to ../b
	// normalize to the "testdata/fs/a"
	if err := makeFs(tmpdir, []dirOrLink{
		{path: "testdata/fs/a/e", target: "../b"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/a/e", "testdata/fs/a/b", "testdata/fs/a"); err != nil {
		t.Fatal(err)
	}

	// avoid letting symlink -> ../directory/file escape from scope
	// normalize to "testdata/fs/j"
	if err := makeFs(tmpdir, []dirOrLink{{path: "testdata/fs/j/k", target: "../i/a"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "testdata/fs/j/k", "testdata/fs/j/i/a", "testdata/fs/j"); err != nil {
		t.Fatal(err)
	}

	// make sure we don't allow escaping to /
	// normalize to dir
	if err := makeFs(tmpdir, []dirOrLink{{path: "foo", target: "/"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "foo", "", ""); err != nil {
		t.Fatal(err)
	}

	// make sure we don't allow escaping to /
	// normalize to dir
	if err := makeFs(filepath.Join(tmpdir, "dir", "subdir"), []dirOrLink{{path: "foo", target: "/../../"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "foo", "", ""); err != nil {
		t.Fatal(err)
	}

	// make sure we stay in scope without leaking information
	// this also checks for escaping to /
	// normalize to dir
	if err := makeFs(filepath.Join(tmpdir, "dir", "subdir"), []dirOrLink{{path: "foo", target: "../../"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "foo", "", ""); err != nil {
		t.Fatal(err)
	}

	if err := makeFs(tmpdir, []dirOrLink{{path: "bar/foo", target: "baz/target"}}); err != nil {
		t.Fatal(err)
	}
	if err := testSymlink(tmpdir, "bar/foo", "bar/baz/target", ""); err != nil {
		t.Fatal(err)
	}
}
