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
	idGitHub       = 104
	idLoad         = 105
	idSearch       = 106
	idOutput       = 110
	idOutputBrowse = 111
	idThreads      = 120
	idExtract      = 121
	idCancel       = 122
	wmRunUI        = wmApp + 1
	projectURL     = "https://github.com/help660vip/LitePayloadDumper"
	projectLabel   = projectURL
)

type application struct {
	hwnd     uintptr
	font     uintptr
	instance uintptr

	titleLabel       uintptr
	subtitleLabel    uintptr
	githubLink       uintptr
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
	partitionList    uintptr
	outputLabel      uintptr
	outputEdit       uintptr
	outputButton     uintptr
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
	largeIcon := loadIconResource(instance, 1, 32, 32)
	smallIcon := loadIconResource(instance, 1, 16, 16)
	defer func() {
		if largeIcon != 0 {
			procDestroyIcon.Call(largeIcon)
		}
		if smallIcon != 0 && smallIcon != largeIcon {
			procDestroyIcon.Call(smallIcon)
		}
	}()
	className := wstr("LitePayloadDumperWindow")
	wc := wndClassEx{
		Size:       uint32(unsafe.Sizeof(wndClassEx{})),
		Style:      0x0003,
		WndProc:    wndProcCallback,
		Instance:   instance,
		Cursor:     cursor,
		Background: colorBtnFace + 1,
		ClassName:  className,
		Icon:       largeIcon,
		IconSmall:  smallIcon,
	}
	if result, _, _ := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); result == 0 {
		showMessage(0, "LitePayloadDumper", "无法注册窗口类。", mbOK|mbIconError)
		return
	}

	app := &application{instance: instance, uiActions: make(map[uintptr]func())}
	currentApp = app
	width, height := int32(1000), int32(780)
	screenW, _, _ := procGetSystemMetrics.Call(0)
	screenH, _, _ := procGetSystemMetrics.Call(1)
	x := (int32(screenW) - width) / 2
	y := (int32(screenH) - height) / 2
	hwnd := createWindow(wsExAcceptFiles, "LitePayloadDumperWindow", "LitePayloadDumper", wsOverlappedWindow|wsClipChildren, x, y, width, height, 0, 0, instance)
	if hwnd == 0 {
		showMessage(0, "LitePayloadDumper", "无法创建主窗口。", mbOK|mbIconError)
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
		if app.handleEditShortcut(&message) {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	if app.font != 0 {
		procDeleteObject.Call(app.font)
	}
}

func (app *application) handleEditShortcut(message *msg) bool {
	if message == nil || message.Message != wmKeyDown || message.WParam != vkA {
		return false
	}
	state, _, _ := procGetKeyState.Call(vkControl)
	if !app.isSelectAllShortcut(message, uint16(state)&0x8000 != 0) {
		return false
	}
	sendMessage(message.Hwnd, emSetSel, 0, ^uintptr(0))
	return true
}

func (app *application) isSelectAllShortcut(message *msg, controlDown bool) bool {
	return controlDown && message != nil && message.Message == wmKeyDown && message.WParam == vkA && app.isEditableControl(message.Hwnd)
}

func (app *application) isEditableControl(hwnd uintptr) bool {
	return hwnd != 0 && (hwnd == app.inputEdit || hwnd == app.searchEdit || hwnd == app.outputEdit || hwnd == app.threadEdit)
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
		case wmDrawItem:
			item := (*drawItemStruct)(unsafe.Pointer(lParam))
			if item != nil && item.ControlID == idGitHub {
				drawGitHubLink(item, app.font)
				return 1
			}
		case wmSetCursor:
			if wParam == app.githubLink {
				cursor, _, _ := procLoadCursor.Call(0, idcHand)
				procSetCursor.Call(cursor)
				return 1
			}
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
