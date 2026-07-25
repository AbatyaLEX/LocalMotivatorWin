//go:build windows

package windows

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"
	"runtime"

	syswindows "golang.org/x/sys/windows"
)

const (
	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004
	nifInfo    = 0x00000010

	niifInfo = 0x00000001

	idiInformation = 32516

	wmDestroy = 0x0002
	wmUser    = 0x0400

	notificationCallbackMessage = wmUser + 1
	notificationID              = 1

	tooltipText = "Local Motivator"

	className  = "LocalMotivatorHiddenWindow"
	windowName = "Local Motivator"
)

type point struct {
	x int32
	y int32
}

type message struct {
	hWnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	location point
}

type windowClassExW struct {
	cbSize        uint32
	style         uint32
	windowProc    uintptr
	clsExtra      int32
	wndExtra      int32
	instance      uintptr
	icon          uintptr
	cursor        uintptr
	background    uintptr
	menuName      *uint16
	className     *uint16
	iconSmall     uintptr
}

type notifyIconDataW struct {
	cbSize            uint32
	hWnd              uintptr
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             uintptr
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          [16]byte
	hBalloonIcon      uintptr
}

var (
	shell32   = syswindows.NewLazySystemDLL("shell32.dll")
	user32    = syswindows.NewLazySystemDLL("user32.dll")
	kernel32  = syswindows.NewLazySystemDLL("kernel32.dll")

	shellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	loadIconW          = user32.NewProc("LoadIconW")
	registerClassExW   = user32.NewProc("RegisterClassExW")
	createWindowExW    = user32.NewProc("CreateWindowExW")
	defWindowProcW     = user32.NewProc("DefWindowProcW")
	destroyWindow      = user32.NewProc("DestroyWindow")
	getMessageW        = user32.NewProc("GetMessageW")
	translateMessage   = user32.NewProc("TranslateMessage")
	dispatchMessageW   = user32.NewProc("DispatchMessageW")
	postQuitMessage    = user32.NewProc("PostQuitMessage")

	getModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	notificationMutex sync.Mutex
	notificationData  notifyIconDataW

	windowHandle uintptr
	iconAdded    bool
	started      bool
	startError   error
	startOnce    sync.Once
	ready        = make(chan struct{})
)

func Start() error {
	startOnce.Do(func() {
		go runMessageLoop()
	})

	<-ready

	return startError
}

func ShowNotification(title, text string) error {
	if err := Start(); err != nil {
		return err
	}

	notificationMutex.Lock()
	defer notificationMutex.Unlock()

	copyUTF16(notificationData.szInfoTitle[:], title)
	copyUTF16(notificationData.szInfo[:], text)

	notificationData.uFlags = nifInfo
	notificationData.dwInfoFlags = niifInfo

	if err := callShellNotifyIcon(
		nimModify,
		&notificationData,
	); err != nil {
		return fmt.Errorf("show Windows notification: %w", err)
	}

	return nil
}

func Close() {
	notificationMutex.Lock()
	defer notificationMutex.Unlock()

	if iconAdded {
		_ = callShellNotifyIcon(
			nimDelete,
			&notificationData,
		)

		iconAdded = false
	}

	if windowHandle != 0 {
		destroyWindow.Call(windowHandle)
		windowHandle = 0
	}
}

func runMessageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := createHiddenWindow(); err != nil {
		startError = err
		close(ready)
		return
	}

	if err := addNotificationIcon(); err != nil {
		startError = err
		close(ready)
		return
	}

	started = true

	close(ready)

	var msg message

	for {
		result, _, callErr := getMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0,
			0,
			0,
		)

		if int32(result) == -1 {
			if callErr != nil &&
				callErr != syswindows.ERROR_SUCCESS {
				return
			}

			return
		}

		if result == 0 {
			return
		}

		translateMessage.Call(
			uintptr(unsafe.Pointer(&msg)),
		)

		dispatchMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
		)
	}
}

func createHiddenWindow() error {
	instance, _, callErr := getModuleHandleW.Call(0)
	if instance == 0 {
		return fmt.Errorf(
			"get module handle: %w",
			callErr,
		)
	}

	classNameUTF16, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return fmt.Errorf("encode class name: %w", err)
	}

	windowNameUTF16, err := syscall.UTF16PtrFromString(windowName)
	if err != nil {
		return fmt.Errorf("encode window name: %w", err)
	}

	windowClass := windowClassExW{
		cbSize:     uint32(unsafe.Sizeof(windowClassExW{})),
		windowProc: syscall.NewCallback(windowProcedure),
		instance:   instance,
		className:  classNameUTF16,
	}

	classAtom, _, registerErr := registerClassExW.Call(
		uintptr(unsafe.Pointer(&windowClass)),
	)
	if classAtom == 0 {
		return fmt.Errorf(
			"register hidden window class: %w",
			registerErr,
		)
	}

	handle, _, createErr := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNameUTF16)),
		uintptr(unsafe.Pointer(windowNameUTF16)),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		instance,
		0,
	)
	if handle == 0 {
		return fmt.Errorf(
			"create hidden window: %w",
			createErr,
		)
	}

	windowHandle = handle

	return nil
}

func addNotificationIcon() error {
	icon := loadInformationIcon()
	if icon == 0 {
		return errors.New("load information icon")
	}

	notificationMutex.Lock()
	defer notificationMutex.Unlock()

	notificationData = notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             windowHandle,
		uID:              notificationID,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: notificationCallbackMessage,
		hIcon:            icon,
		dwInfoFlags:      niifInfo,
	}

	copyUTF16(
		notificationData.szTip[:],
		tooltipText,
	)

	if err := callShellNotifyIcon(
		nimAdd,
		&notificationData,
	); err != nil {
		return fmt.Errorf("add notification icon: %w", err)
	}

	iconAdded = true

	return nil
}

func windowProcedure(
	hWnd uintptr,
	message uint32,
	wParam uintptr,
	lParam uintptr,
) uintptr {
	switch message {
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}

	result, _, _ := defWindowProcW.Call(
		hWnd,
		uintptr(message),
		wParam,
		lParam,
	)

	return result
}

func loadInformationIcon() uintptr {
	icon, _, _ := loadIconW.Call(
		0,
		uintptr(idiInformation),
	)

	return icon
}

func callShellNotifyIcon(
	command uint32,
	data *notifyIconDataW,
) error {
	result, _, callErr := shellNotifyIconW.Call(
		uintptr(command),
		uintptr(unsafe.Pointer(data)),
	)

	if result != 0 {
		return nil
	}

	if callErr != nil &&
		callErr != syswindows.ERROR_SUCCESS {
		return callErr
	}

	return errors.New(
		"Shell_NotifyIconW returned zero",
	)
}

func copyUTF16(destination []uint16, text string) {
	clear(destination)

	if len(destination) == 0 {
		return
	}

	encoded := utf16.Encode([]rune(text))

	maxLength := len(destination) - 1
	if len(encoded) > maxLength {
		encoded = encoded[:maxLength]
	}

	copy(destination, encoded)
	destination[len(encoded)] = 0
}