package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rivo/tview"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <serial-port> [functions.json]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s /dev/tty.usbmodem11401\n", os.Args[0])
		os.Exit(1)
	}

	scanner, err := NewScanner(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer func(scanner *Scanner) {
		err := scanner.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(scanner)

	funcFile := "functions.json"
	if len(os.Args) > 2 {
		funcFile = os.Args[2]
	}

	functions, _ := loadFunctions(funcFile)

	app := tview.NewApplication()
	ui := newTUI(app, scanner, functions, funcFile)

	if err := app.SetRoot(ui.layout, true).EnableMouse(true).Run(); err != nil {
		log.Fatal(err)
	}
}

type RawCommand struct {
	cmd string
}

func (c RawCommand) Send() string { return c.cmd }
func (c RawCommand) Parse(response string) (string, error) {
	return response, nil
}
