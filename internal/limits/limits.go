package limits

import (
	"awesomeProject11/internal/domain"

	"fmt"
	"io"
	"net/http"
	"time"
)

const DataLimit = 1024 * 1024 * 1024
const TimeLimit = 1 * time.Hour

type dataTrackingWriter struct {
	user domain.User
	wc   io.WriteCloser
}

type dataTrackingReader struct {
	user domain.User
	rc   io.ReadCloser
}

type NopCloserWriter struct {
	http.ResponseWriter
}

func (n *NopCloserWriter) Close() error { return nil }

func (d *dataTrackingWriter) Write(p []byte) (int, error) {

	if d.user.IsOverDataLimit(DataLimit) {
		return 0, fmt.Errorf("Data limit exceeded")
	}

	n, err := d.wc.Write(p)

	if n > 0 {
		d.user.AddData(int64(n))
	}

	return n, err
}

func (d *dataTrackingReader) Read(p []byte) (int, error) {

	if d.user.IsOverDataLimit(DataLimit) {
		return 0, fmt.Errorf("Data limit exceeded")
	}

	n, err := d.rc.Read(p)
	if n > 0 {

		d.user.AddData(int64(n))

	}
	return n, err
}

func (d *dataTrackingReader) Close() error {
	return d.rc.Close()
}

func (d *dataTrackingWriter) Close() error {
	return d.wc.Close()
}

func NewTrackingWriter(user domain.User, wc io.WriteCloser) io.WriteCloser {
	return &dataTrackingWriter{
		user: user,
		wc:   wc,
	}
}

func NewTrackingReader(user domain.User, rc io.ReadCloser) io.ReadCloser {
	return &dataTrackingReader{
		user: user,
		rc:   rc,
	}
}
