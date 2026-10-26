package main

import (
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Counts
	}{
		{name: "empty", input: "", want: Counts{lines: 0, words: 0, bytes: 0}},
		{name: "simple", input: "hi there\n", want: Counts{lines: 1, words: 2, bytes: 9}},
		{name: "single_line_char", input: "a\nb", want: Counts{lines: 1, words: 2, bytes: 3}},
		// {name: "simple", input: "hi there\n", want: Counts{lines: 1, words: 2, bytes: 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := count(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Unexpceted error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}

}
