package main

import (
	"bufio"
	"errors"
	"flag"
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
	l := flag.Bool("l", false, "print only lines")
	w := flag.Bool("w", false, "print only word count")
	c := flag.Bool("c", false, "print only size")
	flag.Parse()

	if !*l && !*w && !*c {
		*l, *w, *c = true, true, true
	}

	r := os.Stdin
	counts, err := count(r)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *l {
		fmt.Printf("%8d", counts.lines)
	}
	if *w {
		fmt.Printf("%8d", counts.words)
	}
	if *c {
		fmt.Printf("%8d", counts.bytes)
	}

	// if !*l && !*w && !*c {
	// 	fmt.Printf("%8d%8d%8d ", counts.lines, counts.words, counts.bytes)
	// }
	fmt.Println()

}
