// This file holds a filesystem backend.
// So pgmig can use native filesystem (by default) or embedded filesystem (which can be set in New).

package pgmig

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

type defaultFS struct{}

func (fs defaultFS) Walk(path string, wf filepath.WalkFunc) error {
	// Walk does not follow symbolic links.
	//	return filepath.Walk(path, wf)

	d, err := fs.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	files, err := d.Readdir(-1)
	if err != nil {
		return err
	}
	for _, fi := range files {
		if fi.Mode().IsRegular() {
			err := wf(path, fi, err)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Open like http.FileSystem's Open
func (fs defaultFS) Open(name string) (http.File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err // TODO: What with mapDirOpenError(err, fullName)?
	}
	return f, nil
}

// ReadFile reads file via filesystem method
func (fs defaultFS) ReadFile(name string) (string, error) {
	f, err := fs.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	s := string(b)
	return s, nil
}
