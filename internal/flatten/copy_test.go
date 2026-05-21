package flatten

import (
	"errors"
	"syscall"
	"testing"
)

func TestWriteAllRetriesShortWrites(t *testing.T) {
	oldWrite := syscallWrite
	defer func() { syscallWrite = oldWrite }()

	var chunks [][]byte
	syscallWrite = func(fd int, p []byte) (int, error) {
		if len(p) > 3 {
			p = p[:3]
		}
		chunks = append(chunks, append([]byte(nil), p...))
		return len(p), nil
	}

	if err := writeAll(1, []byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}

	got := ""
	for _, chunk := range chunks {
		got += string(chunk)
	}
	if got != "abcdefgh" {
		t.Fatalf("wrote %q", got)
	}
}

func TestWriteAllRetriesEAGAIN(t *testing.T) {
	oldWrite := syscallWrite
	defer func() { syscallWrite = oldWrite }()

	calls := 0
	syscallWrite = func(fd int, p []byte) (int, error) {
		calls++
		if calls == 1 {
			return 0, syscall.EAGAIN
		}
		return len(p), nil
	}

	if err := writeAll(1, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestWriteAllReturnsNonRetryableError(t *testing.T) {
	oldWrite := syscallWrite
	defer func() { syscallWrite = oldWrite }()

	want := errors.New("boom")
	syscallWrite = func(fd int, p []byte) (int, error) {
		return 0, want
	}

	if err := writeAll(1, []byte("abc")); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
