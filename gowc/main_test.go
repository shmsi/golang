package main

import (
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Counts
	}{
		{name: "empty", input: "", want: Counts{lines: 0, words: 0, bytes: 0, characters: 0}},
		{name: "simple", input: "hi there\n", want: Counts{lines: 1, words: 2, bytes: 9, characters: 9}},
		{name: "no_trailing_newline", input: "a\nb", want: Counts{lines: 1, words: 2, bytes: 3, characters: 3}},
		{name: "only_newlines", input: "\n\n\n", want: Counts{lines: 3, words: 0, bytes: 3, characters: 3}},
		{name: "tab", input: "a\tb\n", want: Counts{lines: 1, words: 2, bytes: 4, characters: 4}},
		{name: "lead_and_trail", input: "  lead and trail  \n", want: Counts{lines: 1, words: 3, bytes: 19, characters: 19}},
		{name: "swedish", input: "Åsa är här\n", want: Counts{lines: 1, words: 3, bytes: 14, characters: 11}},
		{name: "emoji", input: "hej 🙂\n", want: Counts{lines: 1, words: 2, bytes: 9, characters: 6}},
		{name: "multibyte_at_end", input: "kö", want: Counts{lines: 0, words: 1, bytes: 3, characters: 2}},
		// Invalid UTF-8: each bad byte counts as one character (utf8.RuneError, size 1).
		{name: "invalid_byte", input: "a\xffb", want: Counts{lines: 0, words: 1, bytes: 3, characters: 3}},
		// Input ends in the middle of a 2-byte rune: the lone first byte is one invalid character.
		{name: "truncated_rune", input: "a\xc3", want: Counts{lines: 0, words: 1, bytes: 2, characters: 2}},
	}

	// Each case runs through several readers. strings.NewReader returns all the
	// data in one Read; the iotest wrappers split it up, so a multi-byte rune can
	// land across two Reads, and the last data can arrive together with io.EOF.
	readers := []struct {
		name string
		wrap func(io.Reader) io.Reader
	}{
		{name: "plain", wrap: func(r io.Reader) io.Reader { return r }},
		{name: "onebyte", wrap: iotest.OneByteReader},
		{name: "half", wrap: iotest.HalfReader},
		{name: "dataerr", wrap: iotest.DataErrReader},
	}

	for _, tt := range tests {
		for _, rd := range readers {
			t.Run(tt.name+"/"+rd.name, func(t *testing.T) {
				got, err := count(rd.wrap(strings.NewReader(tt.input)))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %+v, want %+v", got, tt.want)
				}
			})
		}
	}
}

func BenchmarkCount(b *testing.B) {
	input := strings.Repeat("hej Åsa är här\n", 100_000)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		_, _ = count(strings.NewReader(input))
	}
}
