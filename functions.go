package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

func SetSquelch(s *Scanner, level int) error {
	if level < 0 || level > 15 {
		return fmt.Errorf("squelch level must be 0-15, got %d", level)
	}
	cmds := []KEYCommand{
		{Key: Func, Mode: KeyPress},
		{Key: RotaryPush, Mode: KeyPress},
	}
	for i := 0; i < 15; i++ {
		cmds = append(cmds, KEYCommand{Key: RotaryRight, Mode: KeyPress})
	}
	for i := 0; i < 15-level; i++ {
		cmds = append(cmds, KEYCommand{Key: RotaryLeft, Mode: KeyPress})
	}
	cmds = append(cmds, KEYCommand{Key: RotaryPush, Mode: KeyPress})

	for _, cmd := range cmds {
		if _, err := Execute(s, cmd); err != nil {
			return err
		}
	}
	return nil
}

var placeholderRe = regexp.MustCompile(`\{(\w+)}`)

type Function struct {
	Name     string   `json:"name"`
	Commands []string `json:"commands"`
}

func loadFunctions(path string) ([]Function, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fns []Function
	return fns, json.Unmarshal(data, &fns)
}

func extractPlaceholders(commands []string) []string {
	seen := map[string]bool{}
	var ordered []string
	for _, cmd := range commands {
		for _, match := range placeholderRe.FindAllStringSubmatch(cmd, -1) {
			name := match[1]
			if !seen[name] {
				seen[name] = true
				ordered = append(ordered, name)
			}
		}
	}
	return ordered
}

func substituteValues(cmd string, values map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(cmd, func(match string) string {
		name := match[1 : len(match)-1]
		if v, ok := values[name]; ok {
			return v
		}
		return match
	})
}
