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
	lines int
	words int
	bytes int
}

func (c *Counts) add(other Counts) {
	c.bytes += other.bytes
	c.lines += other.lines
	c.words += other.words
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

func countFile(path string) (Counts, error) {
	f, err := os.Open(path)
	if err != nil {
		return Counts{}, err
	}
	defer f.Close()
	return count(f)
}

func printCounts(counts Counts, name string, showLines, showWords, showBytes bool) {
	if !showLines && !showWords && !showBytes {
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
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {

		r := os.Stdin
		counts, err := count(r)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		printCounts(counts, "", *l, *w, *c)
	} else {
		var total Counts
		var wg sync.WaitGroup
		results := make([]fileResult, len(args))
		for i, arg := range args {
			wg.Add(1)
			go func() {
				defer wg.Done()
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
			printCounts(r, args[i], *l, *w, *c)
		}
		if len(args) > 1 {
			printCounts(total, "total", *l, *w, *c)
		}
		if anyFailed {
			os.Exit(1)
		}

	}

}
