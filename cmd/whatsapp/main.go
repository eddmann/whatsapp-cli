package main

import (
	"fmt"
	"os"

	"github.com/eddmann/whatsapp-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if cli.IsJSON() {
			fmt.Fprintf(os.Stderr, `{"error":%q}`+"\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}
