package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

	layout    tview.Primitive
	console   *tview.TextView
	input     *tview.InputField
	cmdList   *tview.List
	funcList  *tview.List
	infoPanel *tview.TextView
	pages     *tview.Pages

	psiMu      sync.Mutex
	psiCancel  context.CancelFunc
	psiActive  atomic.Bool
	psiInterval atomic.Int32
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

	ui.cmdList = tview.NewList().ShowSecondaryText(true)
	ui.cmdList.SetBorder(true).SetTitle(" Commands ")
	ui.cmdList.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite))
	ui.populateCmdList()

	ui.funcList = tview.NewList().ShowSecondaryText(true)
	ui.funcList.SetBorder(true).SetTitle(fmt.Sprintf(" Functions (%s) ", ui.funcFile))
	ui.funcList.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite))
	ui.populateFuncList(functions)

	ui.infoPanel = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	ui.infoPanel.SetBorder(true).SetTitle(" Scanner Info ")
	fmt.Fprint(ui.infoPanel, "[::d]Run GSI to populate[-]")

	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.cmdList, 0, 1, false).
		AddItem(ui.funcList, 0, 1, false)

	centerPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.console, 0, 1, false).
		AddItem(ui.input, 3, 0, true)

	mainFlex := tview.NewFlex().
		AddItem(leftPanel, 36, 0, false).
		AddItem(centerPanel, 0, 1, true).
		AddItem(ui.infoPanel, 42, 0, false)

	ui.pages = tview.NewPages().AddPage("main", mainFlex, true, true)
	ui.layout = ui.pages

	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			switch {
			case ui.cmdList.HasFocus():
				ui.app.SetFocus(ui.funcList)
			case ui.funcList.HasFocus():
				ui.app.SetFocus(ui.input)
			default:
				ui.app.SetFocus(ui.cmdList)
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

func (ui *TUI) populateCmdList() {
	type entry struct {
		name string
		desc string
		run  func()
	}

	builtins := []entry{
		{
			name: "VERCommand",
			desc: "VER — get firmware version",
			run: func() {
				if ui.running.Load() {
					return
				}
				go func() {
					ui.setBlocked(true)
					defer ui.setBlocked(false)
					ui.mu.Lock()
					defer ui.mu.Unlock()
					ui.logf("[yellow]> VER[-]\n")
					r, err := Execute(ui.scanner, VERCommand{})
					if err != nil {
						ui.logf("[red]Error: %v[-]\n", err)
						return
					}
					ui.logf("Version: %s\n", tview.Escape(r.Version))
				}()
			},
		},
		{
			name: "MDLCommand",
			desc: "MDL — get model info",
			run: func() {
				if ui.running.Load() {
					return
				}
				go func() {
					ui.setBlocked(true)
					defer ui.setBlocked(false)
					ui.mu.Lock()
					defer ui.mu.Unlock()
					ui.logf("[yellow]> MDL[-]\n")
					r, err := Execute(ui.scanner, MDLCommand{})
					if err != nil {
						ui.logf("[red]Error: %v[-]\n", err)
						return
					}
					ui.logf("Model: %s\n", tview.Escape(r.Model))
				}()
			},
		},
		{
			name: "KEYCommand",
			desc: "KEY,{key},{mode} — send keypress",
			run: func() {
				if ui.running.Load() {
					return
				}
				go ui.triggerKEYCommand()
			},
		},
		{
			name: "CopyConsole",
			desc: "Copy console text to clipboard",
			run: func() {
				text := ui.console.GetText(true)
				cmd := exec.Command("pbcopy")
				cmd.Stdin = strings.NewReader(text)
				if err := cmd.Run(); err != nil {
					ui.logf("[red]Copy failed: %v[-]\n", err)
					return
				}
				ui.logf("[green]Console copied to clipboard[-]\n")
			},
		},
		{
			name: "GSICommand",
			desc: "GSI — get scanner info (updates right panel)",
			run: func() {
				if ui.running.Load() {
					return
				}
				go func() {
					ui.setBlocked(true)
					defer ui.setBlocked(false)
					ui.mu.Lock()
					defer ui.mu.Unlock()
					ui.logf("[yellow]> GSI[-]\n")
					info, err := ExecuteAll(ui.scanner, GSICommand{})
					if err != nil {
						ui.logf("[red]Error: %v[-]\n", err)
						return
					}
					ui.logf("[green]GSI OK[-]\n")
					ui.app.QueueUpdateDraw(func() {
						ui.infoPanel.Clear()
						fmt.Fprint(ui.infoPanel, renderGSIPanel(info))
						ui.infoPanel.ScrollToBeginning()
					})
				}()
			},
		},
		{
			name: "PSI Start",
			desc: "PSI,<ms> — push scanner info periodically",
			run: func() {
				if ui.running.Load() || ui.psiActive.Load() {
					return
				}
				go ui.triggerPSIStart()
			},
		},
		{
			name: "PSI Stop",
			desc: "PSI,0 — stop periodic push",
			run: func() {
				ui.stopPSI()
			},
		},
	}

	for _, b := range builtins {
		ui.cmdList.AddItem(b.name, b.desc, 0, b.run)
	}
}

type keyResult struct {
	key  string
	mode string
	ok   bool
}

func (ui *TUI) triggerKEYCommand() {
	done := make(chan keyResult, 1)

	ui.app.QueueUpdateDraw(func() {
		form := tview.NewForm()
		form.AddInputField("Key: ", "", 10, nil, nil)
		form.AddInputField("Mode (P/H/R): ", "P", 10, nil, nil)

		send := func() {
			keyField, _ := form.GetFormItem(0).(*tview.InputField)
			modeField, _ := form.GetFormItem(1).(*tview.InputField)
			select {
			case done <- keyResult{keyField.GetText(), modeField.GetText(), true}:
			default:
			}
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
		}
		cancel := func() {
			select {
			case done <- keyResult{ok: false}:
			default:
			}
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
		}

		form.AddButton("Send", send)
		form.AddButton("Cancel", cancel)
		form.SetBorder(true).SetTitle(" KEYCommand ")
		form.SetCancelFunc(cancel)

		modal := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(form, 14, 0, true).
				AddItem(nil, 0, 1, false), 44, 0, true).
			AddItem(nil, 0, 1, false)

		ui.pages.AddPage("modal", modal, true, true)
		ui.app.SetFocus(form)
	})

	result := <-done
	if !result.ok || result.key == "" {
		return
	}

	ui.setBlocked(true)
	defer ui.setBlocked(false)
	ui.mu.Lock()
	defer ui.mu.Unlock()

	k := Key(strings.ToUpper(result.key))
	m := KeyMode(strings.ToUpper(result.mode))
	ui.logf("[yellow]> KEY,%s,%s[-]\n", k, m)
	_, err := Execute(ui.scanner, KEYCommand{Key: k, Mode: m})
	if err != nil {
		ui.logf("[red]Error: %v[-]\n", err)
		return
	}
	ui.logf("[green]KEY sent[-]\n")
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
	if err := ui.scanner.Send(cmd); err != nil {
		ui.logf("[red]Error: %v[-]\n", err)
		return
	}
	response, err := ui.scanner.ReadAll()
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
	if err := ui.scanner.Send(cmd); err != nil {
		ui.logf("[red]Error: %v[-]\n", err)
		return
	}
	response, err := ui.scanner.ReadAll()
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

func (ui *TUI) triggerPSIStart() {
	type psiResult struct {
		interval int
		ok       bool
	}
	done := make(chan psiResult, 1)

	ui.app.QueueUpdateDraw(func() {
		form := tview.NewForm()
		form.AddInputField("Interval (ms): ", "300", 10, nil, nil)

		submit := func() {
			field, _ := form.GetFormItem(0).(*tview.InputField)
			val := 300
			fmt.Sscanf(field.GetText(), "%d", &val)
			select {
			case done <- psiResult{val, true}:
			default:
			}
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
		}
		cancel := func() {
			select {
			case done <- psiResult{ok: false}:
			default:
			}
			ui.pages.RemovePage("modal")
			ui.app.SetFocus(ui.input)
		}

		form.AddButton("Start", submit)
		form.AddButton("Cancel", cancel)
		form.SetBorder(true).SetTitle(" PSI Interval ")
		form.SetCancelFunc(cancel)

		modal := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(form, 10, 0, true).
				AddItem(nil, 0, 1, false), 38, 0, true).
			AddItem(nil, 0, 1, false)

		ui.pages.AddPage("modal", modal, true, true)
		ui.app.SetFocus(form)
	})

	result := <-done
	if !result.ok {
		return
	}
	ui.startPSI(result.interval)
}

func (ui *TUI) startPSI(interval int) {
	ui.psiMu.Lock()
	defer ui.psiMu.Unlock()

	if ui.psiCancel != nil {
		ui.psiCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	ui.psiCancel = cancel
	ui.psiInterval.Store(int32(interval))
	ui.psiActive.Store(true)

	go ui.psiLoop(ctx, interval)
}

func (ui *TUI) stopPSI() {
	ui.psiMu.Lock()
	defer ui.psiMu.Unlock()

	if ui.psiCancel == nil {
		return
	}
	ui.psiCancel()
	ui.psiCancel = nil
}

func (ui *TUI) psiLoop(ctx context.Context, interval int) {
	defer func() {
		ui.psiActive.Store(false)
		ui.app.QueueUpdateDraw(func() {
			ui.infoPanel.SetTitle(" Scanner Info ")
		})
		ui.logf("[::d]PSI stopped[-]\n")
	}()

	send := func() bool {
		ui.mu.Lock()
		defer ui.mu.Unlock()
		err := ui.scanner.Send(fmt.Sprintf("PSI,%d", interval))
		if err != nil {
			ui.logf("[red]PSI send error: %v[-]\n", err)
			return false
		}
		return true
	}

	if !send() {
		return
	}
	ui.logf("[yellow]> PSI,%d[-]\n[green]PSI active[-]\n", interval)
	ui.app.QueueUpdateDraw(func() {
		ui.infoPanel.SetTitle(fmt.Sprintf(" Scanner Info [PSI:%dms] ", interval))
	})

	lastSent := time.Now()

	for {
		// Auto-restart before 2-min timeout.
		if time.Since(lastSent) > 110*time.Second {
			if !send() {
				return
			}
			lastSent = time.Now()
			ui.logf("[::d]PSI refreshed[-]\n")
		}

		// Read one push. ReadAll blocks until 100ms silence so it
		// naturally waits for a complete XML document before returning.
		ui.mu.Lock()
		data, err := ui.scanner.ReadAll()
		ui.mu.Unlock()

		select {
		case <-ctx.Done():
			ui.mu.Lock()
			_ = ui.scanner.Send("PSI,0")
			ui.mu.Unlock()
			return
		default:
		}

		if err != nil || data == "" {
			continue
		}

		info, err := (GSICommand{}).Parse(data)
		if err != nil {
			continue
		}

		rendered := renderGSIPanel(info)
		ui.app.QueueUpdateDraw(func() {
			row, col := ui.infoPanel.GetScrollOffset()
			ui.infoPanel.Clear()
			fmt.Fprint(ui.infoPanel, rendered)
			ui.infoPanel.ScrollTo(row, col)
			ui.infoPanel.SetTitle(fmt.Sprintf(" Scanner Info [PSI:%dms] ", interval))
		})
	}
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
