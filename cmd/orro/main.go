// Command orro controls Tuya-based standing desks from the terminal.
package main

import (
	"fmt"
	"os"

	"github.com/yashiels/orro-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "orro: %v\n", err)
		os.Exit(1)
	}
}
