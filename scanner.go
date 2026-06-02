package main

import (
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Scanner struct {
	port serial.Port
}

func NewScanner(portName string) (*Scanner, error) {
	port, err := serial.Open(portName, &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", portName, err)
	}
	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		err := port.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("failed to set read timeout: %w", err)
	}
	return &Scanner{port: port}, nil
}

func (s *Scanner) Close() error { return s.port.Close() }

func (s *Scanner) Send(line string) error {
	_, err := fmt.Fprintf(s.port, "%s\r", line)
	return err
}

func (s *Scanner) ReadLine() (string, error) {
	var buf strings.Builder
	b := make([]byte, 1)
	for {
		_, err := s.port.Read(b)
		if err != nil {
			return "", err
		}
		if b[0] == '\r' {
			return buf.String(), nil
		}
		buf.WriteByte(b[0])
	}
}
