package main

import "fmt"

func main() {
	name := "DelveUI"
	count := 0
	for i := 1; i <= 5; i++ {
		count += i
		msg := fmt.Sprintf("Step %d: count=%d, name=%s", i, count, name)
		fmt.Println(msg)
	}
	fmt.Printf("Done. Total: %d\n", count)
}
