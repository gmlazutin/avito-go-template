package middleware

import (
	"net/http"
	"sync"
)

type responseRecorder struct {
	http.ResponseWriter

	mu          sync.Mutex
	status      int
	wroteHeader bool
	err         error
	panicStack  []byte
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (w *responseRecorder) WriteHeader(status int) {
	w.mu.Lock()
	if w.wroteHeader {
		w.mu.Unlock()
		return
	}
	w.status = status
	w.wroteHeader = true
	w.mu.Unlock()
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(payload []byte) (int, error) {
	w.mu.Lock()
	if !w.wroteHeader {
		w.wroteHeader = true
		w.status = http.StatusOK
	}
	w.mu.Unlock()
	return w.ResponseWriter.Write(payload)
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseRecorder) RecordError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.err = err
}

func (w *responseRecorder) recordPanic(err error, stack []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.err = err
	w.panicStack = append([]byte(nil), stack...)
	if !w.wroteHeader {
		w.status = http.StatusInternalServerError
	}
}

func (w *responseRecorder) snapshot() (status int, err error, stack []byte, wroteHeader bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status, w.err, append([]byte(nil), w.panicStack...), w.wroteHeader
}

func recorderFrom(w http.ResponseWriter) *responseRecorder {
	for w != nil {
		if recorder, ok := w.(*responseRecorder); ok {
			return recorder
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil
		}
		w = unwrapper.Unwrap()
	}
	return nil
}
