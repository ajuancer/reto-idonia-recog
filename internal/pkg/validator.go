package pkg

import (
	"bytes"
	"errors"
	"io"
)

type FileType string

const (
	TypePDF     FileType = "pdf"
	TypeDICOM   FileType = "dicom"
	TypeUnknown FileType = "unknown"
)

// DetectFileType reads the magic bytes of a file to determine its true format.
func DetectFileType(reader io.Reader) (FileType, error) {
	// 132 bytes (128 preamble + "DICM")
	header := make([]byte, 132)
	n, err := io.ReadFull(reader, header)

	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return TypeUnknown, err
	}

	if n < 5 {
		return TypeUnknown, errors.New("file is too small to be valid")
	}

	if bytes.HasPrefix(header, []byte("%PDF-")) {
		return TypePDF, nil
	}

	if n >= 132 && bytes.Equal(header[128:132], []byte("DICM")) {
		return TypeDICOM, nil
	}

	return TypeUnknown, nil
}
