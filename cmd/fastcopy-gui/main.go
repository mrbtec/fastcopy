// fastcopy-gui provides a graphical interface for the fastcopy engine
// using the Fyne toolkit.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/moises/fastcopy/internal"
)

const appVersion = "0.2.0"

func main() {
	myApp := app.NewWithID("com.moises.fastcopy")
	myApp.Settings().SetTheme(theme.DarkTheme())

	win := myApp.NewWindow("fastcopy — Ultra-Fast File Copier")
	win.Resize(fyne.NewSize(620, 480))
	win.CenterOnScreen()

	// ── Header ──
	title := widget.NewRichTextFromMarkdown("# ⚡ fastcopy")
	subtitle := widget.NewLabel(fmt.Sprintf("Ultra-fast parallel file copier — v%s", appVersion))
	subtitle.Alignment = fyne.TextAlignCenter

	// ── Source / Destination ──
	srcEntry := widget.NewEntry()
	srcEntry.SetPlaceHolder("Select source directory...")
	srcEntry.Disable()

	dstEntry := widget.NewEntry()
	dstEntry.SetPlaceHolder("Select destination directory...")
	dstEntry.Disable()

	srcBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			srcEntry.SetText(uri.Path())
		}, win)
	})

	dstBtn := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			dstEntry.SetText(uri.Path())
		}, win)
	})

	srcRow := container.NewBorder(nil, nil, nil, srcBtn, srcEntry)
	dstRow := container.NewBorder(nil, nil, nil, dstBtn, dstEntry)

	pathsForm := container.NewVBox(
		widget.NewLabel("Source:"),
		srcRow,
		widget.NewLabel("Destination:"),
		dstRow,
	)

	// ── Options ──
	chkChecksum := widget.NewCheck("Compute SHA256 checksum", nil)
	chkForce := widget.NewCheck("Force recopy (ignore incremental)", nil)
	chkNoArchive := widget.NewCheck("Don't preserve permissions/timestamps", nil)

	workersEntry := widget.NewEntry()
	workersEntry.SetText("0")
	workersEntry.SetPlaceHolder("0 = auto")
	workersLabel := widget.NewLabel("Workers (0=auto):")
	workersRow := container.NewBorder(nil, nil, workersLabel, nil, workersEntry)

	optionsBox := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Options:"),
		chkChecksum,
		chkForce,
		chkNoArchive,
		workersRow,
	)

	// ── Progress Area ──
	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1
	progressBar.Hide()

	statusLabel := widget.NewLabel("")
	statusLabel.Alignment = fyne.TextAlignCenter

	speedLabel := widget.NewLabel("")
	speedLabel.Alignment = fyne.TextAlignCenter

	filesLabel := widget.NewLabel("")
	filesLabel.Alignment = fyne.TextAlignCenter

	progressBox := container.NewVBox(
		progressBar,
		filesLabel,
		container.NewGridWithColumns(2, speedLabel, statusLabel),
	)

	// ── Log Area ──
	logText := widget.NewMultiLineEntry()
	logText.Disable()
	logText.SetMinRowsVisible(4)
	logScroll := container.NewVScroll(logText)
	logScroll.SetMinSize(fyne.NewSize(0, 100))
	logScroll.Hide()

	appendLog := func(msg string) {
		current := logText.Text
		if current != "" {
			current += "\n"
		}
		logText.SetText(current + msg)
	}

	// ── Copy Button ──
	var copying bool
	var copyBtn *widget.Button
	var stopBtn *widget.Button
	var cancelFunc context.CancelFunc

	copyBtn = widget.NewButton("▶  Start Copy", func() {
		if copying {
			return
		}

		src := srcEntry.Text
		dst := dstEntry.Text

		if src == "" || dst == "" {
			dialog.ShowError(fmt.Errorf("Please select both source and destination directories"), win)
			return
		}

		// Parse workers
		var numWorkers int
		fmt.Sscanf(workersEntry.Text, "%d", &numWorkers)

		opts := internal.Options{
			Archive:  !chkNoArchive.Checked,
			Checksum: chkChecksum.Checked,
			Force:    chkForce.Checked,
		}

		engine := internal.NewCopyEngine(numWorkers, opts, true, false)

		// UI state: running
		copying = true
		copyBtn.Disable()
		stopBtn.Enable()
		srcBtn.Disable()
		dstBtn.Disable()
		progressBar.Show()
		progressBar.SetValue(0)
		logScroll.Show()
		logText.SetText("")
		statusLabel.SetText("Scanning...")
		filesLabel.SetText("")
		speedLabel.SetText("")

		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel

		go func() {
			err := engine.Run(ctx, src, dst)

			// Poll progress while engine is running
			// This goroutine monitors the copy progress
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					p := engine.Progress()
					if p == nil {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					snap := p.Snapshot()

					fraction := float64(0)
					if snap.TotalBytes > 0 {
						fraction = float64(snap.CopiedBytes) / float64(snap.TotalBytes)
					}

					processed := snap.CopiedFiles + snap.SkippedFiles + snap.ErrorFiles

					var etaStr string
					if snap.Speed > 0 && snap.TotalBytes > 0 {
						remaining := float64(snap.TotalBytes-snap.CopiedBytes) / snap.Speed
						if remaining < 0 {
							remaining = 0
						}
						etaStr = internal.FormatDuration(time.Duration(remaining * float64(time.Second)))
					} else {
						etaStr = "..."
					}

					progressBar.SetValue(fraction)
					filesLabel.SetText(fmt.Sprintf("[%d/%d files]  Skipped: %d  Errors: %d",
						processed, snap.TotalFiles,
						snap.SkippedFiles, snap.ErrorFiles))
					speedLabel.SetText(fmt.Sprintf("%s/s", internal.FormatBytes(int64(snap.Speed))))
					statusLabel.SetText(fmt.Sprintf("ETA %s  |  %s / %s",
						etaStr,
						internal.FormatBytes(snap.CopiedBytes),
						internal.FormatBytes(snap.TotalBytes)))

					if processed >= snap.TotalFiles {
						return
					}
					time.Sleep(200 * time.Millisecond)
				}
			}()

			// Wait until engine.Run returns (the err is already captured above)
			// The progress polling goroutine will exit on its own.
			_ = err

			<-done

			// UI state: finished
			copying = false
			copyBtn.Enable()
			stopBtn.Disable()
			srcBtn.Enable()
			dstBtn.Enable()
			progressBar.SetValue(1)

			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || err.Error() == "context canceled" {
					statusLabel.SetText("⚠ Canceled by user")
					appendLog("Operation canceled by user.")
				} else {
					statusLabel.SetText("⚠ Completed with errors")
					appendLog(fmt.Sprintf("ERROR: %s", err))
					for _, e := range engine.Errors() {
						appendLog(fmt.Sprintf("  • %s", e))
					}
				}
			} else {
				p := engine.Progress()
				if p != nil {
					snap := p.Snapshot()
					statusLabel.SetText(fmt.Sprintf("✅ Done — %s copied in %s",
						internal.FormatBytes(snap.CopiedBytes),
						internal.FormatDuration(snap.Elapsed)))
				} else {
					statusLabel.SetText("✅ Done")
				}
			}
		}()
	})

	stopBtn = widget.NewButton("🛑 Stop", func() {
		if cancelFunc != nil {
			cancelFunc()
			stopBtn.Disable()
			appendLog("Stopping copy (waiting for current files to finish)...")
		}
	})
	stopBtn.Disable()

	// ── Layout ──
	content := container.NewVBox(
		container.NewCenter(title),
		container.NewCenter(subtitle),
		widget.NewSeparator(),
		pathsForm,
		optionsBox,
		widget.NewSeparator(),
		container.NewGridWithColumns(2, copyBtn, stopBtn),
		layout.NewSpacer(),
		progressBox,
		logScroll,
	)

	win.SetContent(container.NewPadded(content))
	win.ShowAndRun()
}
