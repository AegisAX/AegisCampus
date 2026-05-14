package middleware

import (
	"net/http"
	"os"
	"strings"
)

// NoIndexFileSystem wraps a http.FileSystem to suppress directory listings.
// Requesting a directory that lacks index.html returns os.ErrPermission,
// which http.FileServer translates to HTTP 403 Forbidden.
//
// Adapted from https://www.alexedwards.net/blog/disable-http-fileserver-directory-listings
type NoIndexFileSystem struct {
	fs http.FileSystem
}

// Open returns a file from the static directory. If the requested path is
// a directory without an index.html file, an os.ErrPermission error is
// returned, causing a 403 Forbidden response.
func (ufs NoIndexFileSystem) Open(name string) (http.File, error) {
	f, err := ufs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	s, err := f.Stat()
	if s.IsDir() {
		index := strings.TrimSuffix(name, "/") + "/index.html"
		_, err := ufs.fs.Open(index)
		if err != nil {
			return nil, os.ErrPermission
		}
	}
	return f, nil
}

// NoIndexDir is a drop-in replacement for http.Dir, providing a filesystem
// that disables directory indexing for serving static files.
func NoIndexDir(path string) http.FileSystem {
	return NoIndexFileSystem{
		fs: http.Dir(path),
	}
}
