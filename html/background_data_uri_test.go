package html

import "testing"

const onePixelPNGDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestBackgroundImageDataURI(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{
			name: "inline style",
			html: `<div style="width: 100px; height: 40px; background-image:url(` + onePixelPNGDataURI + `); background-size: contain"></div>`,
		},
		{
			name: "inline style, quoted url",
			html: `<div style="background-image: url('` + onePixelPNGDataURI + `'); width: 100px; height: 40px"></div>`,
		},
		{
			name: "stylesheet rule",
			html: `<style>.logo { background-image: url(` + onePixelPNGDataURI + `); width: 100px; height: 40px }</style><div class="logo"></div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elems, err := Convert(tt.html, &Options{StrictAssets: true})
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if len(elems) == 0 {
				t.Fatal("expected at least 1 element")
			}
		})
	}
}

func TestResolveBackgroundImageDataURI(t *testing.T) {
	c := &converter{logger: loggerOrDiscard(nil)}
	got := c.resolveBackgroundImage(computedStyle{BackgroundImage: "url(" + onePixelPNGDataURI + ")", FontSize: 12})
	if got == nil {
		t.Fatal("resolveBackgroundImage returned nil for a PNG data URI")
	}
}

func TestInlineStyleKeepsDataURIWhole(t *testing.T) {
	decls := splitDeclarationsCSS("width: 10px; background-image:url(" + onePixelPNGDataURI + "); height: 5px")
	if len(decls) != 3 {
		t.Fatalf("got %d declarations %q, want 3", len(decls), decls)
	}
	if _, val, _ := splitDeclarationWithImportant(decls[1]); val != "url("+onePixelPNGDataURI+")" {
		t.Errorf("background-image value = %q, want the whole data URI", val)
	}
}
