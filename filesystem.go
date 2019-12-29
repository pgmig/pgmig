// This file holds a filesystem backend.
// So pgmig can use native filesystem (by default) or embedded filesystem (which can be set in New).

package pgmig

import (
	"io"
	"os"
	"path/filepath"
)

// FileSystem holds all of used filesystem access methods
type FileSystem interface {
	Walk(root string, walkFn filepath.WalkFunc) error
	Open(name string) (File, error)
}

// File is an interface for os.File struct
type File interface {
	io.Closer
	io.Reader
	io.Seeker
	Readdir(count int) ([]os.FileInfo, error)
	Stat() (os.FileInfo, error)
}

type defaultFS struct{}

func (fs defaultFS) Walk(path string, wf filepath.WalkFunc) error {
	// Walk does not follow symbolic links, by design. Otherwise it is difficult to avoid loops.
	// https://github.com/golang/go/issues/4759#issuecomment-66074349
	// If it does not matter, use
	//  return filepath.Walk(path, wf)

	d, err := fs.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	files, err := d.Readdir(-1)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.Mode().IsRegular() {
			err := wf(path, file, err)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Open like http.FileSystem's Open
func (fs defaultFS) Open(name string) (File, error) { return os.Open(name) }

// ReadFile reads file via filesystem method
//func (fs defaultFS) ReadFile(name string) ([]byte, error) { return ioutil.ReadFile(name) }
