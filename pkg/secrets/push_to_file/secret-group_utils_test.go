package push_to_file

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

//// toWriterPusher
type toWriterPusherArgs struct {
	writer        io.Writer
	groupName     string
	groupTemplate string
	groupSecrets  []*Secret
}

type toWriterPusherSpy struct {
	args   toWriterPusherArgs
	err    error
	_calls int
}

func (spy *toWriterPusherSpy) Call(
	writer io.Writer,
	groupName string,
	groupTemplate string,
	groupSecrets []*Secret,
) error {
	spy._calls++
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = toWriterPusherArgs{
		writer:        writer,
		groupName:     groupName,
		groupTemplate: groupTemplate,
		groupSecrets:  groupSecrets,
	}

	return spy.err
}

//// toWriteCloserOpener
type toWriteCloserOpenerArgs struct {
	path        string
	permissions os.FileMode
}

type toWriteCloserOpenerSpy struct {
	args        toWriteCloserOpenerArgs
	writeCloser io.WriteCloser
	err         error
	_calls      int
}

func (spy *toWriteCloserOpenerSpy) Call(path string, permissions os.FileMode) (io.WriteCloser, error) {
	spy._calls++
	if spy._calls > 1 {
		panic("spy called more than once")
	}

	spy.args = toWriteCloserOpenerArgs{
		path:        path,
		permissions: permissions,
	}

	return spy.writeCloser, spy.err
}
