// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/layout"
)

// makeJPEGBytes returns a small encoded JPEG for tests.
func makeJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestBaseFSLoadsImage verifies that <img> with a relative src is loaded
// from Options.BaseFS instead of the OS filesystem. We assert both that the
// BaseFS was read and that the decoded image made it into the layout.
func TestBaseFSLoadsImage(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"assets/photo.jpg": {Data: jpegData},
	}}

	elems, err := Convert(`<img src="assets/photo.jpg"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement from BaseFS, got %T (alt-text fallback means FS load failed)", elems[0])
	}
	if fsys.count("assets/photo.jpg") == 0 {
		t.Errorf("expected BaseFS to be read; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSLoadsLinkedStylesheet verifies that <link rel="stylesheet"> resolves
// through BaseFS. We check that the stylesheet file was actually read by
// wrapping MapFS in a counting FS.
func TestBaseFSLoadsLinkedStylesheet(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"styles/site.css": {Data: []byte(`p { color: red; }`)},
	}}
	html := `<html><head><link rel="stylesheet" href="styles/site.css"/></head><body><p>hi</p></body></html>`

	if _, err := ConvertFull(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("styles/site.css") == 0 {
		t.Errorf("expected stylesheet to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSLoadsBackgroundImage verifies the background-image: url() resolver
// routes through BaseFS. This path goes through resolveBackgroundImage →
// loadLocalImage, a different code path from <img> and linked stylesheet.
func TestBaseFSLoadsBackgroundImage(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"bg.jpg": {Data: jpegData},
	}}
	html := `<div style="background-image: url('bg.jpg'); width:100px; height:100px;"><p>x</p></div>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("bg.jpg") == 0 {
		t.Errorf("expected background-image to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSTakesPrecedenceOverBasePath verifies that when both BaseFS and
// BasePath are set, BaseFS wins for relative paths.
func TestBaseFSTakesPrecedenceOverBasePath(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := fstest.MapFS{
		"photo.jpg": {Data: jpegData},
	}
	elems, err := Convert(`<img src="photo.jpg"/>`, &Options{
		BaseFS:   fsys,
		BasePath: "/this/path/does/not/exist",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("BaseFS should win over BasePath; got %T", elems[0])
	}
}

// TestBaseFSAbsolutePathFallsThrough verifies that absolute paths bypass BaseFS
// and go to the OS filesystem. This preserves backward compatibility for
// absolute references in CSS/HTML.
func TestBaseFSAbsolutePathFallsThrough(t *testing.T) {
	dir := t.TempDir()
	osPath := dir + "/abs.jpg"
	if err := writeFile(osPath, makeJPEGBytes(t)); err != nil {
		t.Fatal(err)
	}

	// Empty BaseFS — absolute paths should go to OS.
	fsys := fstest.MapFS{}
	elems, err := Convert(fmt.Sprintf(`<img src="%s"/>`, osPath), &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("absolute path should bypass BaseFS; got %T", elems[0])
	}
}

// TestClientUsedForImageFetch verifies that Options.Client is used for HTTP
// image fetches.
func TestClientUsedForImageFetch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(makeJPEGBytes(t))
	}))
	defer srv.Close()

	// Wrap the transport so we can also assert our client is the one making the call.
	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}

	elems, err := Convert(fmt.Sprintf(`<img src="%s/photo.jpg"/>`, srv.URL), &Options{
		Client: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("test server was not hit")
	}
	if seen == 0 {
		t.Error("Options.Client transport was not used for image fetch")
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement, got %T", elems[0])
	}
}

// TestClientUsedForStylesheetFetch verifies that Options.Client is used for
// HTTP <link rel="stylesheet"> loads.
func TestClientUsedForStylesheetFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = io.WriteString(w, `p { color: red; }`)
	}))
	defer srv.Close()

	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}
	html := fmt.Sprintf(`<html><head><link rel="stylesheet" href="%s/s.css"/></head><body><p>hi</p></body></html>`, srv.URL)

	if _, err := ConvertFull(html, &Options{Client: client}); err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Error("Options.Client transport was not used for stylesheet fetch")
	}
}

// TestURLPolicyStillBlocksWithCustomClient confirms that URLPolicy short-circuits
// the fetch before the custom Client is ever consulted.
func TestURLPolicyStillBlocksWithCustomClient(t *testing.T) {
	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}
	_, err := Convert(`<img src="http://example.invalid/x.jpg"/>`, &Options{
		Client:    client,
		URLPolicy: func(string) error { return fmt.Errorf("denied") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Errorf("Client should not have been used when URLPolicy denies: got %d transport calls", seen)
	}
}

// TestBaseFSLoadsFontFace verifies that @font-face src resolves through BaseFS.
// The font bytes are invalid so loadFontFaces silently skips embedding — but
// the counting FS proves readAsset routed the read through BaseFS.
func TestBaseFSLoadsFontFace(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"fonts/fake.ttf": {Data: []byte("not a real font")},
	}}
	html := `<html><head><style>
		@font-face { font-family: "X"; src: url("fonts/fake.ttf"); }
		p { font-family: "X"; }
	</style></head><body><p>hi</p></body></html>`

	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/fake.ttf") == 0 {
		t.Errorf("expected @font-face src to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSRejectsParentEscape verifies that a "../" traversal attempt in
// <img src> against BaseFS fails cleanly (rejected by fs.ValidPath in
// readAsset) and falls back to alt text, not to an OS read outside the FS.
func TestBaseFSRejectsParentEscape(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"safe.jpg": {Data: makeJPEGBytes(t)},
	}}
	elems, err := Convert(`<img src="../etc/passwd" alt="blocked"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Paragraph); !ok {
		t.Fatalf("expected alt-text Paragraph fallback, got %T", elems[0])
	}
	if fsys.count("../etc/passwd") != 0 {
		t.Errorf("fs should not have been opened with traversal path; opens = %v", fsys.snapshot())
	}
}

// TestReadAssetInvalidPath directly exercises readAsset's path handling
// for fs.FS inputs: traversal rejected, leading ./ stripped, not-found is
// a clean fs.ErrNotExist rather than a panic.
func TestReadAssetInvalidPath(t *testing.T) {
	fsys := fstest.MapFS{"dir/a.txt": {Data: []byte("ok")}}

	// ".." traversal must not reach fs.ReadFile.
	if _, err := readAsset(fsys, "", "dir/../../escape"); err == nil {
		t.Error("expected error for traversal path, got nil")
	}

	// Leading "./" is normalized and the read succeeds.
	data, err := readAsset(fsys, "", "./dir/a.txt")
	if err != nil {
		t.Errorf("./ prefix should be stripped: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("unexpected data: %q", data)
	}

	// Missing file surfaces as fs.ErrNotExist (not a panic, not a different error).
	if _, err := readAsset(fsys, "", "dir/nope.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

// TestBasePathStillWorksWithoutBaseFS verifies the legacy path behavior is
// unchanged when BaseFS is nil.
func TestBasePathStillWorksWithoutBaseFS(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(dir+"/legacy.jpg", makeJPEGBytes(t)); err != nil {
		t.Fatal(err)
	}
	elems, err := Convert(`<img src="legacy.jpg"/>`, &Options{BasePath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("BasePath (without BaseFS) should still load image; got %T", elems[0])
	}
}

// countingTransport wraps an http.RoundTripper and counts invocations.
type countingTransport struct {
	inner   http.RoundTripper
	counter *int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*c.counter++
	return c.inner.RoundTrip(req)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// countingFS wraps an fs.FS and records which paths were opened, so a test
// can assert BaseFS was actually the source of a read. Safe for concurrent
// use by Open; snapshots are independent copies.
type countingFS struct {
	inner fstest.MapFS
	mu    sync.Mutex
	opens map[string]int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.mu.Lock()
	if c.opens == nil {
		c.opens = map[string]int{}
	}
	c.opens[name]++
	c.mu.Unlock()
	return c.inner.Open(name)
}

func (c *countingFS) count(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opens[name]
}

func (c *countingFS) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.opens))
	for k, v := range c.opens {
		out[k] = v
	}
	return out
}
