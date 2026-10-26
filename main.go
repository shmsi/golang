package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode"
)

func main() {
	r := bufio.NewReader(os.Stdin)
	lines := 0
	words := 0
	bytes := 0
	inword := false
	for {
		ch, size, err := r.ReadRune()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Println("error")
			break
		}
		bytes += size

		if !unicode.IsSpace(ch) && inword == false {
			words++
			inword = true
		} else if unicode.IsSpace(ch) {
			inword = false
			if ch == '\n' {
				lines++
			}
		}

	}

	fmt.Printf("lines %d and bytes %d and words %d", lines, bytes, words)

}
