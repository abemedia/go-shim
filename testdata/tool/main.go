package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("go-shim-test-tool", os.Args[1:])
	os.Exit(3)
}
