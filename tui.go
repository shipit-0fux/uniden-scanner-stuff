package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TUI struct {
	app      *tview.Application
	scanner  *Scanner
	funcFile string
	mu       sync.Mutex
	running  atomic.Bool

	layout   tview.Primitive
	console  *tview.TextView
	input    *tview.InputField
	funcList *tview.List
	pages    *tview.Pages
}

func newTUI(app *tview.Application, scanner *Scanner, functions []Function, funcFile string) *TUI {
	ui := &TUI{
		app:      app,
		scanner:  scanner,
		funcFile: funcFile,
	}
	ui.build(functions)
	ui.startFileWatcher()
	return ui
}

func (ui *TUI) build(functions []Function) {
	ui.console = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	ui.console.SetBorder(true).SetTitle(" Console ")

	ui.input = tview.NewInputField().
		SetLabel("> ").
		SetFieldBackgroundColor(tcell.ColorDefault)
	ui.input.SetBorder(true)
	ui.input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter || ui.running.Load() {
			return
		}
		cmd := strings.TrimSpace(ui.input.GetText())
		if cmd == "" {
			return
		}
		ui.input.SetText("")
		go ui.executeCommand(cmd)
	})

	ui.funcList = tview.NewList().ShowSecondaryText(true)
	ui.funcList.SetBorder(true).SetTitle(fmt.Sprintf(" Functions (%s) ", ui.funcFile))
	ui.funcList.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite))

	ui.populateFuncList(functions)

	rightPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.console, 0, 1, false).
		AddItem(ui.input, 3, 0, true)

	mainFlex := tview.NewFlex().
		AddItem(ui.funcList, 36, 0, false).
		AddItem(rightPanel, 0, 1, true)

	ui.pages = tview.NewPages().AddPage("main", mainFlex, true, true)
	ui.layout = ui.pages

	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if ui.funcList.HasFocus() {
				ui.app.SetFocus(ui.input)
			} else {
				ui.app.SetFocus(ui.funcList)
			}
			return nil
		case tcell.KeyEscape:
			if !ui.pages.HasPage("modal") {
				ui.app.SetFocus(ui.input)
			}
			return nil
		}
		return event
	})

	_, _ = fmt.Fprint(ui.console, "[::d]Connected to serial port at 115200 baud[-]\n")
	_, _ = fmt.Fprint(ui.console, "[::d]Tab: switch focus | Enter: send/execute | Ctrl+C: quit[-]\n\n")
}

func (ui *TUI) populateFuncList(functions []Function) {
	ui.funcList.Clear()
	for i, fn := range functions {
		var shortcut rune
		if i < 9 {
			shortcut = rune('1' + i)
		}
		preview := strings.Join(fn.Commands, ", ")
		if len(preview) > 28 {
			preview = preview[:25] + "..."
		}
		ui.funcList.AddItem(fn.Name, preview, shortcut, func() {
			if ui.running.Load() {
				return
			}
			go ui.triggerFunction(fn)
		})
	}
	if len(functions) == 0 {
		ui.funcList.AddItem("(no functions loaded)", ui.funcFile, 0, nil)
	}
}

func (ui *TUI) log(text string) {
	ui.app.QueueUpdateDraw(func() {
		_, err := fmt.Fprint(ui.console, text)
		if err != nil {
			ui.log(fmt.Sprintf("QueueUpdateDraw Fprint error: %s", err))
			return
		}
		ui.console.ScrollToEnd()
	})
}

func (ui *TUI) logf(format string, args ...any) {
	ui.log(fmt.Sprintf(format, args...))
}

func (ui *TUI) setBlocked(blocked bool) {
	ui.running.Store(blocked)
	ui.app.QueueUpdateDraw(func() {
		if blocked {
			ui.input.SetLabel("[::d]> [blocked] [::-]")
		} else {
			ui.input.SetLabel("> ")
		}
	})
}

func (ui *TUI) sendCommand(cmd string) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.logf("[yellow]> %s[-]\n", tview.Escape(cmd))
	response, err := Execute(ui.scanner, RawCommand{cmd: cmd})
	if err != nil {
		ui.logf("[red]Error: %v[-]\n", err)
		return
	}
	if response != "" {
		ui.log(tview.Escape(response) + "\n")
	}
}

func (ui *TUI) executeCommand(cmd string) {
	ui.setBlocked(true)
	defer ui.setBlocked(false)
	ui.mu.Lock()
	defer ui.mu.Unlock()

	ui.logf("[yellow]> %s[-]\n", tview.Escape(cmd))
	response, err := Execute(ui.scanner, RawCommand{cmd: cmd})
	if err != nil {
		ui.logf("[red]Error: %v[-]\n", err)
		return
	}
	if response != "" {
		ui.log(tview.Escape(response) + "\n")
	}
}

func (ui *TUI) triggerFunction(fn Function) {
	placeholders := extractPlaceholders(fn.Commands)
	if len(placeholders) == 0 {
		ui.setBlocked(true)
		ui.runFunction(fn, nil)
		ui.setBlocked(false)
		return
	}

	values := make(map[string]string)
	done := make(chan bool, 1)

	ui.app.QueueUpdateDraw(func() {
		form := tview.NewForm()
		for _, p := range placeholders {
			form.AddInputField(p+": ", "", 30, nil, nil)
		}
		form.AddButton("Run", func() {
			for i, p := range placeholders {
				if field, ok := form.GetFormItem(i).(*tview.InputField); ok {
					values[p] = field.GetText()
				}
			}
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
			select {
			case done <- true:
			default:
			}
		})
		form.AddButton("Cancel", func() {
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
			select {
			case done <- false:
			default:
			}
		})
		form.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", fn.Name))
		form.SetCancelFunc(func() {
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
			select {
			case done <- false:
			default:
			}
		})

		height := len(placeholders)*2 + 8
		modal := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(form, height, 0, true).
				AddItem(nil, 0, 1, false), 44, 0, true).
			AddItem(nil, 0, 1, false)

		ui.pages.AddPage("modal", modal, true, true)
		ui.app.SetFocus(form)
	})

	if ok := <-done; ok {
		ui.setBlocked(true)
		ui.runFunction(fn, values)
		ui.setBlocked(false)
	}
}

func (ui *TUI) runFunction(fn Function, values map[string]string) {
	ui.logf("[green::b]▶ %s[::-]\n", tview.Escape(fn.Name))
	for _, cmd := range fn.Commands {
		if values != nil {
			cmd = substituteValues(cmd, values)
		}
		ui.sendCommand(cmd)
	}
	ui.logf("[green]▶ done[-]\n\n")
}

func (ui *TUI) startFileWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		ui.logf("[red]File watcher init error: %v[-]\n", err)
		return
	}
	if err := watcher.Add(ui.funcFile); err != nil {
		err := watcher.Close()
		if err != nil {
			ui.logf("[red]File watcher close error: %v[-]\n", err)
		}
		return
	}
	go func() {
		defer func(watcher *fsnotify.Watcher) {
			err := watcher.Close()
			if err != nil {
				ui.logf("[red]File watcher close error: %v[-]\n", err)
			}
		}(watcher)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					fns, err := loadFunctions(ui.funcFile)
					if err != nil {
						ui.logf("[red]Reload error (%s): %v[-]\n", ui.funcFile, err)
						continue
					}
					ui.app.QueueUpdateDraw(func() {
						ui.populateFuncList(fns)
					})
					ui.logf("[::d]Functions reloaded (%d)[-]\n", len(fns))
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				ui.logf("[red]Watcher error: %v[-]\n", err)
			}
		}
	}()
}
