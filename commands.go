package main

import (
	"fmt"
	"strings"
)

type Command[R any] interface {
	Send() string
	Parse(response string) (R, error)
}

func Execute[R any](s *Scanner, cmd Command[R]) (R, error) {
	if err := s.Send(cmd.Send()); err != nil {
		return zero[R](), err
	}
	line, err := s.ReadLine()
	if err != nil {
		return zero[R](), err
	}
	return cmd.Parse(line)
}

func zero[R any]() R { var z R; return z }

type MDLResponse struct{ Model string }

type MDLCommand struct{}

func (c MDLCommand) Send() string { return "MDL" }
func (c MDLCommand) Parse(response string) (MDLResponse, error) {
	parts := strings.SplitN(response, ",", 2)
	if len(parts) != 2 || parts[0] != "MDL" {
		return MDLResponse{}, fmt.Errorf("unexpected response: %q", response)
	}
	return MDLResponse{Model: parts[1]}, nil
}

type VERResponse struct{ Version string }

type VERCommand struct{}

func (c VERCommand) Send() string { return "VER" }
func (c VERCommand) Parse(response string) (VERResponse, error) {
	parts := strings.SplitN(response, ",", 2)
	if len(parts) != 2 || parts[0] != "VER" {
		return VERResponse{}, fmt.Errorf("unexpected response: %q", response)
	}
	return VERResponse{Version: parts[1]}, nil
}

type Key string

func (k Key) String() string { return string(k) }

const (
	Menu        Key = "M"
	Func        Key = "F"
	Avoid       Key = "L"
	Keypad1     Key = "1"
	Keypad2     Key = "2"
	Keypad3     Key = "3"
	Keypad4     Key = "4"
	Keypad5     Key = "5"
	Keypad6     Key = "6"
	Keypad7     Key = "7"
	Keypad8     Key = "8"
	Keypad9     Key = "9"
	Keypad0     Key = "0"
	KeypadDot   Key = "."
	Enter       Key = "E"
	RotaryRight Key = ">"
	RotaryLeft  Key = "<"
	RotaryPush  Key = "^"
	Backlight   Key = "V"
	System      Key = "Y"
	Department  Key = "B"
	Channel     Key = "C"
	Zip         Key = "Z"
	ServiceType Key = "T"
	Range       Key = "R"
)

type KeyMode string

const (
	KeyPress   KeyMode = "P"
	KeyHold    KeyMode = "H"
	KeyRelease KeyMode = "R"
)

type KEYCommand struct {
	Key  Key
	Mode KeyMode
}

func (c KEYCommand) Send() string { return fmt.Sprintf("KEY,%s,%s", c.Key, c.Mode) }
func (c KEYCommand) Parse(response string) (struct{}, error) {
	if response != "KEY,OK" {
		return struct{}{}, fmt.Errorf("unexpected: %q", response)
	}
	return struct{}{}, nil
}
