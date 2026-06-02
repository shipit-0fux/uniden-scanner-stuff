# Serial Terminal for BCD436HP Scanner

Simple Go application for interacting with BCD436HP scanner via serial port.

## Build

```bash
go build -o serial-term main.go
```

## Usage

```bash
./serial-term /dev/tty.usbmodem11401
```

Replace `/dev/tty.usbmodem11401` with your actual serial port path.

## Commands

Once connected, enter commands like:
- `VER` - Check firmware version
- `STS` - Get scanner status
- `MDL` - Get model info
- Any other BCD436HP command

See `BCD436_1.11.12.pdf` for full command reference.

## Exit

Press `Ctrl+C` to exit cleanly.

## Settings

- Baud rate: 115200
- Data bits: 8
- Parity: None
- Stop bits: 1
- Line ending: CRLF (\r\n)
- Response timeout: 200ms
