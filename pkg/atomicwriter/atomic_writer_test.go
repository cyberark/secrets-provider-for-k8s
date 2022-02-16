package atomicwriter

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyberark/conjur-authn-k8s-client/pkg/log"
	"github.com/stretchr/testify/assert"
)

type assertFunc func(path string, tempFilePath string, t *testing.T, err error)
type errorAssertFunc func(buf *bytes.Buffer, wc io.WriteCloser, t *testing.T, err error)

func TestWriteFile(t *testing.T) {
	testCases := []struct {
		name        string
		path        string
		permissions os.FileMode
		content     string
		assert      assertFunc
	}{
		{
			name:        "happy path",
			path:        "test_file.txt",
			permissions: 0644,
			content:     "test content",
			assert: func(path string, tempFilePath string, t *testing.T, err error) {
				assert.NoError(t, err)
				// Check that the file exists
				assert.FileExists(t, path)
				// Check the contents of the file
				contents, err := ioutil.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, "test content", string(contents))
				// Check the file permissions
				mode, err := os.Stat(path)
				assert.NoError(t, err)
				assert.Equal(t, os.FileMode(0644), mode.Mode())
				// Check that the temp file was deleted
				assert.NoFileExists(t, tempFilePath)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, _ := ioutil.TempDir("", "atomicwriter")
			defer os.RemoveAll(tmpDir)

			path := filepath.Join(tmpDir, tc.path)
			err, tempFilePath := writeFile(path, tc.permissions, []byte(tc.content))
			tc.assert(path, tempFilePath, t, err)
		})
	}
}

func TestWriterAtomicity(t *testing.T) {
	tmpDir, _ := ioutil.TempDir("", "atomicwriter")
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "test_file.txt")

	// Create 2 writers for the same path
	writer1, err := NewAtomicWriter(path, 0600)
	assert.NoError(t, err)
	writer2, err := NewAtomicWriter(path, 0644)
	assert.NoError(t, err)
	// Write different content to each writer
	writer1.Write([]byte("writer 1 line 1\n"))
	writer2.Write([]byte("writer 2 line 1\n"))
	writer1.Write([]byte("writer 1 line 2\n"))
	writer2.Write([]byte("writer 2 line 2\n"))
	// Close the first writer and ensure only the contents of the first writer are written
	err = writer1.Close()

	assert.NoError(t, err)
	// Check that the file exists
	assert.FileExists(t, path)
	// Check the contents of the file match the first writer (which was closed)
	contents, err := ioutil.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "writer 1 line 1\nwriter 1 line 2\n", string(contents))
	// Check the file permissions match the first writer
	mode, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), mode.Mode())
}

func TestLogsErrors(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		osFuncs       osFuncs
		errorOnCreate bool
		assert        errorAssertFunc
	}{
		{
			name:          "nonexistent directory",
			path:          "nonexistent_directory/test_file.txt",
			osFuncs:       stdOSFuncs,
			errorOnCreate: true,
			assert: func(buf *bytes.Buffer, wc io.WriteCloser, t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, buf.String(), "Could not create temporary file")
			},
		},
		{
			name: "unable to remove temporary file",
			path: "test_file.txt",
			osFuncs: osFuncs{
				remove: func(name string) error {
					return os.ErrPermission
				},
				rename: func(oldpath, newpath string) error {
					return os.ErrPermission
				},
				truncate: os.Truncate,
				chmod:    os.Chmod,
			},
			assert: func(buf *bytes.Buffer, wc io.WriteCloser, t *testing.T, err error) {
				assert.Error(t, err)

				// The file should be truncated instead of being deleted
				assert.Contains(t, buf.String(), "Could not delete temporary file")
				assert.Contains(t, buf.String(), "Truncated file")

				// Check that the temp file was truncated
				writer, ok := wc.(*atomicWriter)
				assert.True(t, ok)
				assert.FileExists(t, writer.tempFile.Name())
				content, err := ioutil.ReadFile(writer.tempFile.Name())
				assert.NoError(t, err)
				assert.Equal(t, "", string(content))
			},
		},
		{
			name: "unable to remove or truncate temporary file",
			path: "test_file.txt",
			osFuncs: osFuncs{
				remove: func(name string) error {
					return os.ErrPermission
				},
				rename: func(oldpath, newpath string) error {
					return os.ErrPermission
				},
				truncate: func(name string, size int64) error {
					return os.ErrPermission
				},
				chmod: os.Chmod,
			},
			assert: func(buf *bytes.Buffer, wc io.WriteCloser, t *testing.T, err error) {
				assert.Error(t, err)

				assert.Contains(t, buf.String(), "Could not delete temporary file")
				assert.Contains(t, buf.String(), "File may be left on disk")
			},
		},
		{
			name: "unable to chmod",
			path: "test_file.txt",
			osFuncs: osFuncs{
				remove:   os.RemoveAll,
				rename:   os.Rename,
				truncate: os.Truncate,
				chmod: func(name string, mode os.FileMode) error {
					return os.ErrPermission
				},
			},
			assert: func(buf *bytes.Buffer, wc io.WriteCloser, t *testing.T, err error) {
				assert.NoError(t, err)
				assert.Contains(t, buf.String(), "Could not set permissions on temporary file")

				// Check that the file was still renamed
				writer, ok := wc.(*atomicWriter)
				assert.True(t, ok)
				assert.NoFileExists(t, writer.tempFile.Name())
				assert.FileExists(t, writer.path)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, _ := ioutil.TempDir("", "atomicwriter")
			defer os.RemoveAll(tmpDir)
			path := filepath.Join(tmpDir, tc.path)

			// Mock the logger output
			buf := mockErrorLog()
			defer unmockErrorLog()

			writer, err := newAtomicWriter(path, 0644, tc.osFuncs)

			if tc.errorOnCreate {
				assert.Error(t, err)
				tc.assert(buf, writer, t, err)
				if writer != nil {
					writer.Close()
				}
				return
			}

			assert.NoError(t, err)

			// Try to write the file
			_, err = writer.Write([]byte("test content"))
			assert.NoError(t, err)

			err = writer.Close()
			tc.assert(buf, writer, t, err)
		})
	}
}

func TestDefaultDirectory(t *testing.T) {
	writer, err := NewAtomicWriter("test_file.txt", 0644)
	assert.NoError(t, err)
	defer os.Remove("test_file.txt")

	writer.Write([]byte("test content"))
	err = writer.Close()
	assert.NoError(t, err)
	assert.FileExists(t, "./test_file.txt")
}

func writeFile(path string, permissions os.FileMode, content []byte) (err error, tempFilePath string) {
	writer, err := NewAtomicWriter(path, permissions)
	if err != nil {
		return err, ""
	}

	tempFilePath = writer.(*atomicWriter).tempFile.Name()

	_, err = writer.Write(content)
	if err != nil {
		return err, tempFilePath
	}

	return writer.Close(), tempFilePath
}

// Mocks the logger output to a buffer
func mockErrorLog() *bytes.Buffer {
	buf := &bytes.Buffer{}
	log.ErrorLogger.SetOutput(buf)
	return buf
}

// Unmocks the logger output, sending it back to os.Stderr
func unmockErrorLog() {
	log.ErrorLogger.SetOutput(os.Stderr)
}
