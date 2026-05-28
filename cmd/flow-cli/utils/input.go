// Uses bufio.Scanner on os.Stdin — no external deps.
package utils

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var stdin = bufio.NewScanner(os.Stdin)

func init() {
	stdin.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
}

// Prompt asks for a free-form string. def is the fallback when user hits Enter.
func Prompt(label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	if !stdin.Scan() {
		return def
	}
	s := strings.TrimSpace(stdin.Text())
	if s == "" {
		return def
	}
	return s
}

// Confirm asks a y/N question. def is the default when user hits Enter.
func Confirm(label string, def bool) bool {
	defStr := "y/N"
	if def {
		defStr = "Y/n"
	}
	fmt.Printf("%s [%s]: ", label, defStr)
	if !stdin.Scan() {
		return def
	}
	s := strings.ToLower(strings.TrimSpace(stdin.Text()))
	if s == "" {
		return def
	}
	return s == "y" || s == "yes"
}

// Select prompts the user to pick an index from labels. Returns the picked
// index or -1 when input is invalid / blank.
func Select(prompt string, labels []string) int {
	fmt.Println()
	for i, l := range labels {
		fmt.Printf("  %d) %s\n", i+1, l)
	}
	fmt.Printf("\n%s (1-%d): ", prompt, len(labels))
	if !stdin.Scan() {
		return -1
	}
	s := strings.TrimSpace(stdin.Text())
	if s == "" {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > len(labels) {
		return -1
	}
	return n - 1
}

// PromptInt asks for an integer, returning def on blank/invalid input.
func PromptInt(label string, def int) int {
	s := Prompt(label, strconv.Itoa(def))
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// PromptSecret reads input echoed back to the terminal. (No terminal echo
// suppression — keeping CGO-free; this is the same compromise upstream makes
// outside of its postinstall hook.)
func PromptSecret(label string) string { return Prompt(label, "") }
