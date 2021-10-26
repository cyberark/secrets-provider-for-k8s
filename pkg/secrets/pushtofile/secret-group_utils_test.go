package pushtofile

import (
	"bytes"
	"io"
	"os"
)

type ClosableBuffer struct {
	bytes.Buffer
	CloseErr error
}

func (c ClosableBuffer) Close() error { return c.CloseErr }

//// pushToWriterFunc
type pushToWriterArgs struct {
	writer        io.Writer
	groupName     string
	groupTemplate string
	groupSecrets  []*Secret
}

type pushToWriterSpy struct {
	args   pushToWriterArgs
	err    error
	_calls int
}

func (spy *pushToWriterSpy) Call(
	writer io.Writer,
	groupName string,
	groupTemplate string,
	groupSecrets []*Secret,
) error {
	spy._calls++
	// This is to ensure the spy is only ever used once!
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = pushToWriterArgs{
		writer:        writer,
		groupName:     groupName,
		groupTemplate: groupTemplate,
		groupSecrets:  groupSecrets,
	}

	return spy.err
}

//// openWriteCloserFunc
type openWriteCloserArgs struct {
	path        string
	permissions os.FileMode
}

type openWriteCloserSpy struct {
	args        openWriteCloserArgs
	writeCloser io.WriteCloser
	err         error
	_calls      int
}

func (spy *openWriteCloserSpy) Call(path string, permissions os.FileMode) (io.WriteCloser, error) {
	spy._calls++
	// This is to ensure the spy is only ever used once!
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = openWriteCloserArgs{
		path:        path,
		permissions: permissions,
	}

	return spy.writeCloser, spy.err
}
