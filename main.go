package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-V" || arg == "-v" || arg == "--version" {
			fmt.Printf("automapper version %s\n", version)
			os.Exit(0)
		}
	}

	fmt.Println("Hello, World!")
}
