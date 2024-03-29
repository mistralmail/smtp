package smtp

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

func compare(t *testing.T, data []byte, expected []byte) {
	br := bufio.NewReader(bytes.NewReader(data))

	dataReader := NewDataReader(br)
	output, err := io.ReadAll(dataReader)
	if !bytes.Equal(output, expected) {
		t.Errorf("Expected %v\ngot %v\n", expected, output)
	}
	if err != nil {
		t.Errorf("Did not expect error: %v", err)
	}

}

func expectError(t *testing.T, data []byte, expected error) {
	br := bufio.NewReader(bytes.NewReader(data))
	dataReader := NewDataReader(br)
	_, err := io.ReadAll(dataReader)
	if err != expected {
		t.Errorf("Expected error: %v, got: %v", expected, err)
	}

}

func TestDataReaderValid(t *testing.T) {
	data := []byte("Some test mail\nblablabla\n.\n")
	expected := []byte("Some test mail\nblablabla\n")
	compare(t, data, expected)

	data = []byte("Some test mail\nblablabla\n.\nshould not read this")
	expected = []byte("Some test mail\nblablabla\n")
	compare(t, data, expected)

	data = []byte("Some test mail\n..blablabla\n.\n")
	expected = []byte("Some test mail\n.blablabla\n")
	compare(t, data, expected)

	data = []byte("Some test mail\n.blablabla\n.\n")
	expected = []byte("Some test mail\nblablabla\n")
	compare(t, data, expected)

	// first line is 1000 chars
	data = []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\n.\n")
	expected = []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\n")
	compare(t, data, expected)

	// first line is 1001 chars but starts with a dot, so server should see it as 1000
	data = []byte(".aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsdddddd\n.\n")
	expected = []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsdddddd\n")
	compare(t, data, expected)

	// first line is 1000 chars, second 10, third 1000
	data = []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\naj ge je a t\naafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\n.\n")
	expected = []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\naj ge je a t\naafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd\n")
	compare(t, data, expected)
}

func TestDataReaderInvalid(t *testing.T) {
	data := []byte("Some test mail\nblablabla\nno ending dot")
	expectError(t, data, ErrIncomplete)

	data = []byte("Some test mail\r\nDot on invalid place\n.test")
	expectError(t, data, ErrIncomplete)

	data = []byte("")
	expectError(t, data, ErrIncomplete)
}

func TestDataReaderTooLong(t *testing.T) {
	// length === 1001
	data := []byte("aafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd3\n")
	expectError(t, data, ErrLtl)

	// first line is small, second is 1003
	data = []byte("Some text :)\naafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddddddddfsdaafsddddddd321\n")
	expectError(t, data, ErrLtl)
}
