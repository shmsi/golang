package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan string)

	go func() {
		fmt.Println("sending")
		ch <- "hi"
		fmt.Println("sent")
	}()

	time.Sleep(time.Second)
	fmt.Println("received:", <-ch)

}
