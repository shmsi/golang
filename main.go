package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode"
)

type Counts struct {
	lines int
	words int
	bytes int
}

func count(r io.Reader) (Counts, error) {
	counts := Counts{}
	inword := false
	br := bufio.NewReader(r)
	for {
		ch, size, err := br.ReadRune()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Counts{}, err
		}
		counts.bytes += size

		if !unicode.IsSpace(ch) && !inword {
			counts.words++
			inword = true
		} else if unicode.IsSpace(ch) {
			inword = false
			if ch == '\n' {
				counts.lines++
			}
		}

	}
	return counts, nil
}

func main() {
	r := os.Stdin
	counts, err := count(r)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("lines %d and bytes %d and words %d\n", counts.lines, counts.bytes, counts.words)

}
