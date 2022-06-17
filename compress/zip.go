package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

const zipLevel = 7

func UnZipContent(data []byte) ([]byte, error) {
	if data == nil || string(data) == "{}" {
		return nil, nil
	}

	b := bytes.NewBuffer(data)

	var r io.Reader

	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, fmt.Errorf("UnZipContent gzip.NewReader error: %w", err)
	}

	var resB bytes.Buffer
	if _, err = resB.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("UnZipContent resB.ReadFrom error: %w", err)
	}

	return resB.Bytes(), nil
}

func ZipContent(body []byte) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	var buf bytes.Buffer

	g, err := gzip.NewWriterLevel(&buf, zipLevel)
	if err != nil {
		return nil, fmt.Errorf("ZipContent gzip.NewWriterLevel error: %w", err)
	}

	if _, err := g.Write(body); err != nil {
		return nil, fmt.Errorf("ZipContent gzip write error: %w", err)
	}

	if err = g.Flush(); err != nil {
		return nil, fmt.Errorf("ZipContent gzip Flush error: %w", err)
	}

	if err := g.Close(); err != nil {
		return nil, fmt.Errorf("ZipContent gzip Close error: %w", err)
	}

	return buf.Bytes(), nil
}
