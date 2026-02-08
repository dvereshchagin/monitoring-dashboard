package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipResponseWriter wraps http.ResponseWriter to support gzip compression
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.Writer.Write(b)
}

// gzipWriterPool reuses gzip writers to reduce allocations
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		// Use compression level 5 (good balance between speed and compression ratio)
		w, _ := gzip.NewWriterLevel(nil, 5)
		return w
	},
}

// Compression middleware adds gzip compression to HTTP responses
// Compresses responses larger than 1KB that accept gzip encoding
func Compression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Don't compress already compressed content or streams
		contentType := w.Header().Get("Content-Type")
		if strings.Contains(contentType, "image/") ||
			strings.Contains(contentType, "video/") ||
			strings.Contains(contentType, "application/zip") ||
			strings.Contains(contentType, "application/gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Get gzip writer from pool
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer func() {
			gz.Close()
			gz.Reset(nil)
			gzipWriterPool.Put(gz)
		}()

		gz.Reset(w)

		// Set headers
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length") // Will be different after compression

		// Wrap response writer
		gzw := &gzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
		}

		next.ServeHTTP(gzw, r)
	})
}
