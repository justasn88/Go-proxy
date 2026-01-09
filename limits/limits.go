package limits

import (
	"awesomeProject11/state"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DataLimit = 1024 * 1024 * 1024
const TimeLimit = 1 * time.Hour

type dataTrackingWriter struct {
	user *state.UserState
	wc   io.WriteCloser
}

type dataTrackingReader struct {
	user *state.UserState
	rc   io.ReadCloser
}

type NopCloserWriter struct {
	http.ResponseWriter
}

func (n *NopCloserWriter) Close() error { return nil }

func (d *dataTrackingWriter) Write(p []byte) (int, error) {

	if d.user.IsOverLimit(DataLimit) {
		return 0, fmt.Errorf("Data limit exceeded")
	}

	n, err := d.wc.Write(p)

	if n > 0 {
		d.user.AddData(int64(n))
	}

	return n, err
}

func (d *dataTrackingReader) Read(p []byte) (int, error) {

	d.user.Lock()
	if d.user.IsOverLimit(DataLimit) {
		d.user.Unlock()
		return 0, fmt.Errorf("Data limit exceeded")
	}
	d.user.Unlock()

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

func NewTrackingWriter(user *state.UserState, wc io.WriteCloser) io.WriteCloser {
	return &dataTrackingWriter{
		user: user,
		wc:   wc,
	}
}

func NewTrackingReader(user *state.UserState, rc io.ReadCloser) io.ReadCloser {
	return &dataTrackingReader{
		user: user,
		rc:   rc,
	}
}
