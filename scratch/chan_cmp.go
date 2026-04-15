package main

import "fmt"

func main() {
	ch := make(chan int)
	var rch <-chan int = ch
	fmt.Printf("Matches: %v\n", ch == rch)
}
