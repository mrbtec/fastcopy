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
	idx "github.com/moises/fastcopy/internal/index"
)

const appVersion = "0.2.0"

type AppState struct {
	currentIndex *idx.Index
}

func main() {
	myApp := app.NewWithID("com.moises.fastcopy")
	myApp.Settings().SetTheme(theme.DarkTheme())

	win := myApp.NewWindow("fastcopy — Ultra-Fast File Copier")
	win.Resize(fyne.NewSize(800, 600))
	win.CenterOnScreen()

	state := &AppState{}

	// --- Tab 1: Copier ---
	copierTab := buildCopierTab(win)

	// --- Tab 2: Search ---
	searchTab := buildSearchTab(win, state)

	tabs := container.NewAppTabs(
		container.NewTabItem("Copy Engine", copierTab),
		container.NewTabItem("Index Search", searchTab),
	)

	win.SetContent(tabs)
	win.ShowAndRun()
}

func buildCopierTab(win fyne.Window) fyne.CanvasObject {
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
	chkSkipErrors := widget.NewCheck("Skip errors (continue on permission/read errors)", nil)
	chkRemoveSource := widget.NewCheck("Delete source files after copy (Move)", nil)

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
		chkSkipErrors,
		chkRemoveSource,
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

	startCopy := func() {
		src := srcEntry.Text
		dst := dstEntry.Text

		var numWorkers int
		fmt.Sscanf(workersEntry.Text, "%d", &numWorkers)

		opts := internal.Options{
			Archive:      !chkNoArchive.Checked,
			Checksum:     chkChecksum.Checked,
			Force:        chkForce.Checked,
			SkipErrors:   chkSkipErrors.Checked,
			RemoveSource: chkRemoveSource.Checked,
		}

		engine := internal.NewCopyEngine(numWorkers, opts, true, false)

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

					if processed >= snap.TotalFiles && snap.TotalFiles > 0 {
						return
					}
					time.Sleep(200 * time.Millisecond)
				}
			}()

			err := engine.Run(ctx, src, dst)
			<-done

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
	}

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

		if chkRemoveSource.Checked {
			dialog.ShowConfirm(
				"⚠ Confirmar Deleção da Origem",
				"Os arquivos e pastas da ORIGEM serão DELETADOS após a cópia.\n\n"+
					"Origem: "+src+"\n\n"+
					"Esta operação é IRREVERSÍVEL.\nDeseja continuar?",
				func(confirmed bool) {
					if confirmed {
						startCopy()
					}
				},
				win,
			)
			return
		}

		startCopy()
	})

	stopBtn = widget.NewButton("🛑 Stop", func() {
		if cancelFunc != nil {
			cancelFunc()
			stopBtn.Disable()
			appendLog("Stopping copy (waiting for current files to finish)...")
		}
	})
	stopBtn.Disable()

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

	return container.NewPadded(content)
}

func buildSearchTab(win fyne.Window, state *AppState) fyne.CanvasObject {
	// Index file selector
	idxPathEntry := widget.NewEntry()
	idxPathEntry.SetPlaceHolder("Path to .idx file")
	idxPathEntry.Disable()

	searchBtn := widget.NewButton("Search", nil)
	searchBtn.Disable()
	
	statusLabel := widget.NewLabel("No index loaded")

	idxBrowseBtn := widget.NewButton("Browse", func() {
		dialog.ShowFileOpen(func(uri fyne.URIReadCloser, err error) {
			if err != nil || uri == nil {
				return
			}
			path := uri.URI().Path()
			idxPathEntry.SetText(path)
			
			statusLabel.SetText("Loading index...")
			searchBtn.Disable()
			
			go func() {
				loadedIdx, err := idx.Load(path)
				if err != nil {
					dialog.ShowError(err, win)
					statusLabel.SetText("Failed to load index")
					return
				}
				state.currentIndex = loadedIdx
				statusLabel.SetText(fmt.Sprintf("Index loaded: %d entries", len(loadedIdx.Entries)))
				searchBtn.Enable()
			}()
		}, win)
	})

	// Search controls
	queryEntry := widget.NewEntry()
	queryEntry.SetPlaceHolder("Glob pattern (e.g., *.txt, prefix*) or exact hash")
	dupCheck := widget.NewCheck("Only duplicates", nil)

	// Pagination controls
	pageLabel := widget.NewLabel("Page: 1")
	prevBtn := widget.NewButton("← Prev", nil)
	nextBtn := widget.NewButton("Next →", nil)

	// Results data
	const pageSize = 50
	var results []idx.Entry
	var currentPage int = 0

	// Use List for perfect virtualization and performance
	list := widget.NewList(
		func() int {
			start := currentPage * pageSize
			if start >= len(results) {
				return 0
			}
			end := start + pageSize
			if end > len(results) {
				end = len(results)
			}
			return end - start
		},
		func() fyne.CanvasObject {
			// Template for a single row
			return container.NewGridWithColumns(3,
				widget.NewLabel("Path"),
				widget.NewLabel("Size"),
				widget.NewLabel("Hash"),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			start := currentPage * pageSize
			if start+i >= len(results) {
				return
			}
			entry := results[start+i]
			
			row := o.(*fyne.Container)
			row.Objects[0].(*widget.Label).SetText(entry.Path)
			row.Objects[1].(*widget.Label).SetText(internal.FormatBytes(entry.Size))
			row.Objects[2].(*widget.Label).SetText(entry.Hash)
		},
	)

	// Helper to refresh table data
	refreshView := func() {
		list.Refresh()
		totalPages := (len(results) + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		pageLabel.SetText(fmt.Sprintf("Page: %d / %d", currentPage+1, totalPages))
	}

	searchBtn.OnTapped = func() {
		if state.currentIndex == nil {
			dialog.ShowError(errors.New("please load an index first"), win)
			return
		}

		q := idx.Query{
			Name:       queryEntry.Text,
			Duplicates: dupCheck.Checked,
			// GUI will handle its own pagination via slice slicing, 
			// so we retrieve all matching results to know the total count.
			Limit:      0, 
			Offset:     0,
		}
		
		statusLabel.SetText("Searching...")
		
		go func() {
			start := time.Now()
			res := state.currentIndex.Search(q)
			elapsed := time.Since(start)
			
			// Replace underlying data
			results = res
			currentPage = 0
			
			// UI updates must be on main thread
			statusLabel.SetText(fmt.Sprintf("Found %d results in %v", len(results), elapsed))
			refreshView()
		}()
	}

	// Pagination button actions
	prevBtn.OnTapped = func() {
		if currentPage > 0 {
			currentPage--
			refreshView()
		}
	}
	nextBtn.OnTapped = func() {
		if (currentPage+1)*pageSize < len(results) {
			currentPage++
			refreshView()
		}
	}

	// List header
	header := container.NewGridWithColumns(3,
		widget.NewLabelWithStyle("Path", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Size", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Hash", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	// Assemble layout
	topForm := container.NewBorder(nil, nil, nil, idxBrowseBtn, idxPathEntry)
	controls := container.NewHBox(queryEntry, dupCheck, searchBtn)
	pagination := container.NewHBox(prevBtn, pageLabel, nextBtn)
	content := container.NewBorder(
		container.NewVBox(
			topForm,
			widget.NewSeparator(),
			controls,
			widget.NewSeparator(),
			header,
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewBorder(nil, nil, statusLabel, pagination),
		),
		nil,
		nil,
		list,
	)

	return container.NewPadded(content)
}
