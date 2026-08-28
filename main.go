package main

import (
	"context"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	idInput        = 100
	idBrowse       = 101
	idSelectAll    = 102
	idSelectNone   = 103
	idSelectBoot   = 104
	idLoad         = 105
	idSearch       = 106
	idOutput       = 110
	idOutputBrowse = 111
	idSource       = 112
	idSourceBrowse = 113
	idThreads      = 120
	idExtract      = 121
	idCancel       = 122
	wmRunUI        = wmApp + 1
)

type application struct {
	hwnd     uintptr
	font     uintptr
	instance uintptr

	titleLabel       uintptr
	subtitleLabel    uintptr
	inputLabel       uintptr
	inputEdit        uintptr
	loadButton       uintptr
	browseButton     uintptr
	infoGroup        uintptr
	infoLabels       [8]uintptr
	infoValues       [8]uintptr
	partitionLabel   uintptr
	searchLabel      uintptr
	searchEdit       uintptr
	selectAllButton  uintptr
	selectNoneButton uintptr
	selectBootButton uintptr
	partitionList    uintptr
	outputLabel      uintptr
	outputEdit       uintptr
	outputButton     uintptr
	sourceLabel      uintptr
	sourceEdit       uintptr
	sourceButton     uintptr
	threadLabel      uintptr
	threadEdit       uintptr
	extractButton    uintptr
	cancelButton     uintptr
	statusLabel      uintptr
	progressBar      uintptr
	logEdit          uintptr

	details           *PackageDetails
	partitions        []PartitionItem
	visiblePartitions []PartitionItem
	partitionChecks   map[string]bool
	busy              bool
	cancel            context.CancelFunc
	closeAfterCancel  bool
	loadSerial        uint64

	uiMu      sync.Mutex
	uiNext    uintptr
	uiActions map[uintptr]func()
}

var (
	currentApp      *application
	wndProcCallback = syscall.NewCallback(windowProc)
)

func main() {
	runtime.LockOSThread()
	runApplication()
}

func runApplication() {
	procSetProcessDPIAware.Call()
	controls := initCommonControlsEx{Size: uint32(unsafe.Sizeof(initCommonControlsEx{})), ICC: 0x21}
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&controls)))
	instance, _, _ := procGetModuleHandle.Call(0)
	cursor, _, _ := procLoadCursor.Call(0, idcArrow)
	className := wstr("LitePayloadDumperWindow")
	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		Style:      0x0003,
		WndProc:    wndProcCallback,
		Instance:   instance,
		Cursor:     cursor,
		Background: colorBtnFace + 1,
		ClassName:  className,
	}
	if result, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
		showMessage(0, "Payload 分区提取器", "无法注册窗口类。", mbOK|mbIconError)
		return
	}

	app := &application{instance: instance, uiActions: make(map[uintptr]func())}
	currentApp = app
	width, height := int32(1000), int32(780)
	screenW, _, _ := procGetSystemMetrics.Call(0)
	screenH, _, _ := procGetSystemMetrics.Call(1)
	x := (int32(screenW) - width) / 2
	y := (int32(screenH) - height) / 2
	hwnd := createWindow(wsExAcceptFiles, "LitePayloadDumperWindow", "LitePayloadDumper - Payload 分区提取器", wsOverlappedWindow|wsClipChildren, x, y, width, height, 0, 0, instance)
	if hwnd == 0 {
		showMessage(0, "Payload 分区提取器", "无法创建主窗口。", mbOK|mbIconError)
		return
	}
	app.hwnd = hwnd
	app.font = createUIFont()
	app.createControls()
	app.layout()
	procDragAcceptFiles.Call(hwnd, 1)
	procShowWindow.Call(hwnd, swShow)
	procUpdateWindow.Call(hwnd)
	procSetFocus.Call(app.inputEdit)

	var message msg
	for {
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	if app.font != 0 {
		procDeleteObject.Call(app.font)
	}
}

func windowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	app := currentApp
	if app != nil {
		switch message {
		case wmSize:
			app.layout()
			return 0
		case wmCommand:
			app.handleCommand(int(loword(wParam)), int(hiword(wParam)))
			return 0
		case wmDropFiles:
			if filename := draggedFile(wParam); filename != "" && !app.busy {
				app.loadFile(filename)
			}
			return 0
		case wmGetMinMax:
			limits := (*minMaxInfo)(unsafe.Pointer(lParam))
			limits.MinTrackSize = point{X: 900, Y: 720}
			return 0
		case wmRunUI:
			app.runQueuedUI(wParam)
			return 0
		case wmClose:
			if app.busy {
				if showMessage(hwnd, "确认退出", "正在处理固件。是否取消任务并在清理后退出？", mbYesNo|mbIconQuestion) == idYes {
					app.closeAfterCancel = true
					app.cancelExtraction()
				}
				return 0
			}
			procDestroyWindow.Call(hwnd)
			return 0
		case wmDestroy:
			if app.cancel != nil {
				app.cancel()
			}
			if app.details != nil {
				app.details.Cleanup()
			}
			procPostQuitMessage.Call(0)
			return 0
		}
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func (app *application) queueUI(action func()) {
	app.uiMu.Lock()
	app.uiNext++
	id := app.uiNext
	app.uiActions[id] = action
	app.uiMu.Unlock()
	postMessage(app.hwnd, wmRunUI, id, 0)
}

func (app *application) runQueuedUI(id uintptr) {
	app.uiMu.Lock()
	action := app.uiActions[id]
	delete(app.uiActions, id)
	app.uiMu.Unlock()
	if action != nil {
		action()
	}
}
