package instagram

import "io"

type progressWriter struct {
	reader io.Reader
	total  int64
	read   int64
	onProg func(read, total int64)
}

func (pw *progressWriter) Read(p []byte) (n int, err error) {
	n, err = pw.reader.Read(p)
	pw.read += int64(n)

	if pw.onProg != nil {
		pw.onProg(pw.read, pw.total)
	}
	return n, err
}
