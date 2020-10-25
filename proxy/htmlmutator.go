package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"

	"github.com/diamondburned/ghproxy/htmlmut"
	"github.com/pkg/errors"
)

// HTMLMutator provides a wrapper around ResponseWriter to mutate HTML.
type HTMLMutator struct {
	mutator htmlmut.MutateFunc
}

func NewHTMLMutator(mutator htmlmut.MutateFunc) HTMLMutator {
	return HTMLMutator{
		mutator: mutator,
	}
}

func (hmu HTMLMutator) NewWriter(w http.ResponseWriter) *HTMLMutatorWriter {
	return &HTMLMutatorWriter{
		ResponseWriter: w,
		mutator:        hmu,
	}
}

type HTMLMutatorWriter struct {
	http.ResponseWriter
	mutator     HTMLMutator
	buffer      bytes.Buffer
	statusCode  int
	rawData     []byte // decompressed
	compression string
}

func (muwr *HTMLMutatorWriter) WriteHeader(statusCode int) {
	header := muwr.ResponseWriter.Header()
	// Nobody likes this.
	header.Del("Content-Security-Policy")
	// This is unpredictable. We can rely on chunked encoding.
	header.Del("Content-Length")

	// Finalize.
	muwr.ResponseWriter.WriteHeader(statusCode)

	// Try and flush the chunk.
	if flusher, ok := muwr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	// // We cannot reliably write the header here, as we need to change the
	// // Content-Length header.
	// muwr.statusCode = statusCode
}

func (muwr *HTMLMutatorWriter) Write(b []byte) (int, error) {
	return muwr.buffer.Write(b)
}

func (muwr *HTMLMutatorWriter) readUncompressed() error {
	muwr.compression = muwr.Header().Get("Content-Encoding")
	if muwr.compression == "" {
		muwr.rawData = muwr.buffer.Bytes()
		return nil
	}

	var reader io.Reader

	switch muwr.compression {
	case "gzip": // GitHub uses this.
		gzipR, err := gzip.NewReader(&muwr.buffer)
		if err != nil {
			return errors.Wrap(err, "failed to create gzip reader")
		}
		defer gzipR.Close()

		reader = gzipR

	case "deflate":
		// Error is safe to ignore as DefaultCompression is within the
		// range.
		flateR := flate.NewReader(&muwr.buffer)
		defer flateR.Close()
		reader = flateR

	default:
		return fmt.Errorf("unknown encoding %q", muwr.compression)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return errors.Wrap(err, "failed to decompress")
	}

	muwr.rawData = buf.Bytes()
	return nil
}

// ApplyHTML applies the mutator and flushes the body.
func (muwr *HTMLMutatorWriter) ApplyHTML() error {
	if err := muwr.readUncompressed(); err != nil {
		return err
	}

	body := muwr.mutator.mutator(muwr.rawData)

	// // Write the header with the changed Content-Length.
	// muwr.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(body)))
	// muwr.ResponseWriter.WriteHeader(muwr.statusCode)

	// Create a writer so we can easily override this.
	writer := io.Writer(muwr.ResponseWriter)
	closer := func() error { return nil }

	// Check if we need to compress everything.
	switch muwr.compression {
	case "gzip": // GitHub uses this.
		gzipW := gzip.NewWriter(writer)
		closer = gzipW.Close
		writer = gzipW

	case "deflate":
		// Error is safe to ignore as DefaultCompression is within the
		// range.
		flateW, _ := flate.NewWriter(writer, flate.DefaultCompression)
		closer = flateW.Close
		writer = flateW
	}

	if _, err := writer.Write(body); err != nil {
		return errors.Wrap(err, "failed to write")
	}

	if err := closer(); err != nil {
		return errors.Wrap(err, "failed to flush compressor")
	}

	return nil
}
