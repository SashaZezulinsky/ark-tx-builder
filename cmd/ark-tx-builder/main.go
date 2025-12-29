package main

import (
	"fmt"
	"os"
)

const version = "1.0.0"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Printf("ark-tx-builder version %s\n", version)
		fmt.Println("Deterministic Bitcoin transaction builders for the Ark protocol")
		return
	}

	fmt.Println("Ark Transaction Builder")
	fmt.Println("======================")
	fmt.Println()
	fmt.Println("This is a library for building deterministic Bitcoin transactions for the Ark protocol.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  Import the library: import \"github.com/utexo/ark-tx-builders\"")
	fmt.Println("  See README.md for API documentation and examples")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -v, --version    Show version information")
	fmt.Println("  -h, --help       Show this help message")
}
