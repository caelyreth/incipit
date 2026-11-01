// Package media describes the image formats accepted and stored by Neverland.
package media

import (
	"path/filepath"
	"slices"
	"strings"
)

// Format separates accepted input aliases from the format kept on disk.
type Format struct {
	Decoder         string
	Extensions      []string
	StoredMIME      string
	StoredExtension string
	NormalizePNG    bool
}

var formats = []Format{
	{Decoder: "png", Extensions: []string{"png"}, StoredMIME: "image/png", StoredExtension: "png"},
	{Decoder: "jpeg", Extensions: []string{"jpg", "jpeg"}, StoredMIME: "image/jpeg", StoredExtension: "jpg"},
	{Decoder: "gif", Extensions: []string{"gif"}, StoredMIME: "image/gif", StoredExtension: "gif"},
	{Decoder: "webp", Extensions: []string{"webp"}, StoredMIME: "image/webp", StoredExtension: "webp"},
	{Decoder: "bmp", Extensions: []string{"bmp"}, StoredMIME: "image/bmp", StoredExtension: "bmp"},
	{Decoder: "tiff", Extensions: []string{"tif", "tiff"}, StoredMIME: "image/png", StoredExtension: "png", NormalizePNG: true},
}

// DecoderFormat returns the storage format for an image decoder's name.
func DecoderFormat(name string) (Format, bool) {
	for _, format := range formats {
		if format.Decoder == name {
			format.Extensions = slices.Clone(format.Extensions)
			return format, true
		}
	}
	return Format{}, false
}

// Extensions are accepted input file extensions without dots, sorted for
// dialogs. TIFF is accepted even though its bytes are stored as PNG.
func Extensions() []string {
	var extensions []string
	for _, format := range formats {
		extensions = append(extensions, format.Extensions...)
	}
	slices.Sort(extensions)
	return extensions
}

// AcceptsFile reports whether path has an accepted image-file extension.
func AcceptsFile(path string) bool {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	for _, format := range formats {
		if slices.Contains(format.Extensions, extension) {
			return true
		}
	}
	return false
}

// StoredExtension returns the canonical extension for a stored MIME type.
func StoredExtension(mime string) string {
	for _, format := range formats {
		if format.StoredMIME == mime {
			return format.StoredExtension
		}
	}
	return "bin"
}
