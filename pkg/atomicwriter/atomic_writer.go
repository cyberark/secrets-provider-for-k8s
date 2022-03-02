package atomicwriter

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

// OS Function table
type osFuncs struct {
	chmod    func(string, os.FileMode) error
	rename   func(string, string) error
	remove   func(string) error
	truncate func(string, int64) error
}

// Instantiation of OS Function table using std OS
var stdOSFuncs = osFuncs{
	chmod:    os.Chmod,
	rename:   os.Rename,
	remove:   os.Remove,
	truncate: os.Truncate,
}

type atomicWriter struct {
	path        string
	permissions os.FileMode
	tempFile    *os.File
	os          osFuncs
}

// This package provides a simple atomic file writer which implements the
// io.WriteCloser interface. This allows us to use AtomicWriter the way we
// would use any other Writer, such as a Buffer. Additonally, this struct
// takes the file path during construction, so the code which calls
// `Write()` doesn't need to be concerned with the destination, just like
// any other writer.
func NewAtomicWriter(path string, permissions os.FileMode) (io.WriteCloser, error) {
	return newAtomicWriter(path, permissions, stdOSFuncs)
}

func newAtomicWriter(path string, permissions os.FileMode, osFuncs osFuncs) (io.WriteCloser, error) {
	dir, file := filepath.Split(path)

	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		log.Error(messages.CSPFK055E, path)
		return nil, err
	}

	return &atomicWriter{
		path:        path,
		tempFile:    f,
		permissions: permissions,
		os:          osFuncs,
	}, nil
}

func (w *atomicWriter) Write(content []byte) (n int, err error) {
	// Write to the temporary file
	return w.tempFile.Write(content)
}

func (w *atomicWriter) Close() error {
	defer w.Cleanup()

	// Flush and close the temporary file
	err := w.tempFile.Sync()
	if err != nil {
		log.Error(messages.CSPFK056E, w.tempFile.Name())
		return err
	}
	w.tempFile.Close()

	// Set the file permissions
	err = w.os.chmod(w.tempFile.Name(), w.permissions)
	if err != nil {
		log.Error(messages.CSPFK057E, w.tempFile.Name())
		// Try to rename the file anyway
	}

	// Rename the temporary file to the destination
	err = w.os.rename(w.tempFile.Name(), w.path)
	if err != nil {
		log.Error(messages.CSPFK058E, w.tempFile.Name(), w.path)
		return err
	}

	return nil
}

// Cleanup attempts to remove the temporary file. This function is called by
// the `Close()` method, but can also be called manually in cases where `Close()`
// is not called.
func (w *atomicWriter) Cleanup() {
	err := w.os.remove(w.tempFile.Name())
	if err == nil {
		return
	}

	// If we can't remove the temporary directory, truncate the file to remove all secret content
	err = w.os.truncate(w.tempFile.Name(), 0)
	if err == nil || os.IsNotExist(err) {
		log.Error(messages.CSPFK059E, w.tempFile.Name(), w.path)
		return
	}

	// If that failed as well, log the error
	log.Error(messages.CSPFK060E, w.tempFile.Name(), w.path)
}
