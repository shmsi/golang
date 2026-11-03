package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"unicode"
)

type Counts struct {
	lines      int
	words      int
	bytes      int
	characters int
}

func (c *Counts) add(other Counts) {
	c.bytes += other.bytes
	c.lines += other.lines
	c.words += other.words
	c.characters += other.characters
}

func count(r io.Reader) (Counts, error) {
	counts := Counts{}
	inword := false
	br := bufio.NewReader(r)
	for {
		ch, size, err := br.ReadRune()

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Counts{}, err
		}
		counts.bytes += size
		counts.characters++
		isSpace := unicode.IsSpace(ch)

		if !isSpace && !inword {
			counts.words++
			inword = true
		} else if isSpace {
			inword = false
			if ch == '\n' {
				counts.lines++
			}
		}

	}
	return counts, nil
}

func countFile(path string) (Counts, error) {
	f, err := os.Open(path)
	if err != nil {
		return Counts{}, err
	}
	defer f.Close()
	return count(f)
}

func printCounts(counts Counts, name string, showLines, showWords, showBytes bool, showChars bool) {
	if !showLines && !showWords && !showBytes && !showChars {
		showLines, showWords, showBytes = true, true, true
	}

	if showLines {
		fmt.Printf("%8d", counts.lines)
	}
	if showWords {
		fmt.Printf("%8d", counts.words)
	}
	if showBytes {
		fmt.Printf("%8d", counts.bytes)
	}
	if showChars {
		fmt.Printf("%8d", counts.characters)
	}

	if name != "" {
		fmt.Printf(" %s", name)
	}

	fmt.Println()
}

type fileResult struct {
	counts Counts
	err    error
}

func main() {
	l := flag.Bool("l", false, "print only lines")
	w := flag.Bool("w", false, "print only word count")
	c := flag.Bool("c", false, "print only size")
	m := flag.Bool("m", false, "Print number of chars")
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {

		r := os.Stdin
		counts, err := count(r)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		printCounts(counts, "", *l, *w, *c, *m)
	} else {
		var total Counts
		var wg sync.WaitGroup
		sem := make(chan struct{}, 3)
		results := make([]fileResult, len(args))
		for i, arg := range args {
			wg.Add(1)
			go func() {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				result, err := countFile(arg)
				results[i] = fileResult{result, err}
			}()
		}
		wg.Wait()

		anyFailed := false

		for i, fr := range results {
			r, err := fr.counts, fr.err
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				anyFailed = true

				continue
			}
			total.add(r)
			printCounts(r, args[i], *l, *w, *c, *m)
		}
		if len(args) > 1 {
			printCounts(total, "total", *l, *w, *c, *m)
		}
		if anyFailed {
			os.Exit(1)
		}

	}

}
