package media

import (
	"slices"
	"strings"
	"testing"
)

func TestAcceptedExtensionsAndStoredFormats(t *testing.T) {
	tests := []struct {
		decoder   string
		aliases   []string
		mime      string
		extension string
		normalize bool
	}{
		{"png", []string{"png"}, "image/png", "png", false},
		{"jpeg", []string{"jpg", "jpeg"}, "image/jpeg", "jpg", false},
		{"gif", []string{"gif"}, "image/gif", "gif", false},
		{"webp", []string{"webp"}, "image/webp", "webp", false},
		{"bmp", []string{"bmp"}, "image/bmp", "bmp", false},
		{"tiff", []string{"tif", "tiff"}, "image/png", "png", true},
	}
	var extensions []string
	for _, tt := range tests {
		format, ok := DecoderFormat(tt.decoder)
		if !ok || !slices.Equal(format.Extensions, tt.aliases) || format.StoredMIME != tt.mime ||
			format.StoredExtension != tt.extension || format.NormalizePNG != tt.normalize {
			t.Errorf("decoder %q: got %+v, found %v", tt.decoder, format, ok)
		}
		if got := StoredExtension(tt.mime); got != tt.extension {
			t.Errorf("stored MIME %q: got %q, want %q", tt.mime, got, tt.extension)
		}
		for _, alias := range tt.aliases {
			if !AcceptsFile("/Pictures/PHOTO."+alias) || !AcceptsFile("/Pictures/PHOTO."+strings.ToUpper(alias)) {
				t.Errorf("file extension %q was not accepted case-insensitively", alias)
			}
		}
		extensions = append(extensions, tt.aliases...)
	}
	slices.Sort(extensions)
	if got := Extensions(); !slices.Equal(got, extensions) {
		t.Fatalf("dialog extensions = %v, want %v", got, extensions)
	}
	if _, ok := DecoderFormat("unknown"); ok || AcceptsFile("image.svg") || AcceptsFile("image") {
		t.Fatal("unknown formats must not be accepted")
	}
	if got := StoredExtension("application/octet-stream"); got != "bin" {
		t.Errorf("unknown MIME extension = %q, want bin", got)
	}
}

func TestFormatMetadataIsNotMutableByCallers(t *testing.T) {
	format, _ := DecoderFormat("jpeg")
	format.Extensions[0] = "changed"
	extensions := Extensions()
	extensions[0] = "changed"
	if !AcceptsFile("photo.jpg") || AcceptsFile("photo.changed") {
		t.Fatal("caller changed shared format metadata")
	}
}
