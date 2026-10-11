package win

import (
	"fmt"
	"unsafe"
)

// The tray icon is the flower too: the same pixels the window shows, shrunk to
// 16x16 and handed to the shell. It is the one place a hidden app still exists,
// so it is also where the menu lives.

// IconFromRGBA turns straight RGBA into an HICON the shell will draw: a 32bpp
// colour bitmap plus the all-zero mask that means "use my alpha".
func IconFromRGBA(rgba []uint8, size int) (HICON, error) {
	if len(rgba) < size*size*4 {
		return 0, fmt.Errorf("win: IconFromRGBA wants %d bytes, got %d", size*size*4, len(rgba))
	}
	header := BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       int32(size),
		BiHeight:      -int32(size),
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
	}
	var colorBits unsafe.Pointer
	color, _, err := pCreateDIBSection.Call(
		0,
		uintptr(unsafe.Pointer(&BITMAPINFO{Header: header})),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&colorBits)),
		0, 0,
	)
	if color == 0 {
		return 0, fmt.Errorf("win: icon colour bitmap: %v", err)
	}
	// straight BGRA, not premultiplied: an icon's alpha is its own business
	dst := unsafe.Slice((*uint8)(colorBits), size*size*4)
	for i := 0; i < size*size; i++ {
		dst[i*4+0] = rgba[i*4+2]
		dst[i*4+1] = rgba[i*4+1]
		dst[i*4+2] = rgba[i*4+0]
		dst[i*4+3] = rgba[i*4+3]
	}
	// the mask is all zeroes, which tells Windows the alpha channel is in charge
	mask, _, _ := pCreateBitmap.Call(uintptr(size), uintptr(size), 1, 1, 0)
	if mask == 0 {
		pDeleteObject.Call(color)
		return 0, fmt.Errorf("win: icon mask bitmap failed")
	}
	info := ICONINFO{FIcon: 1, HbmColor: HBITMAP(color), HbmMask: HBITMAP(mask)}
	icon, _, err := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	// the icon copies the bitmaps, so ours can go now
	pDeleteObject.Call(color)
	pDeleteObject.Call(mask)
	if icon == 0 {
		return 0, fmt.Errorf("win: CreateIconIndirect: %v", err)
	}
	return HICON(icon), nil
}

// Tray is a shell tray icon with a menu behind it.
type Tray struct {
	hwnd  HWND
	id    UINT
	icon  HICON
	added bool
}

// AddTray puts an icon in the notification area. The window it belongs to gets
// WM_TRAYICON when the icon is clicked or right-clicked.
func AddTray(hwnd HWND, icon HICON, tip string) (*Tray, error) {
	t := &Tray{hwnd: hwnd, id: 1, icon: icon}
	data := NOTIFYICONDATAW{
		CbSize:           DWORD(unsafe.Sizeof(NOTIFYICONDATAW{})),
		Hwnd:             hwnd,
		UID:              t.id,
		UFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP | NIF_SHOWTIP,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            icon,
	}
	copyUTF16(data.SzTip[:], tip)
	ret, _, err := pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&data)))
	if ret == 0 {
		return nil, fmt.Errorf("win: Shell_NotifyIcon failed: %v", err)
	}
	// ask the shell for the modern behaviour (dismissal, positioning)
	data.UVersion = 4
	pShellNotifyIconW.Call(NIM_SETVERSION, uintptr(unsafe.Pointer(&data)))
	t.added = true
	return t, nil
}

// SetIcon swaps the picture in the tray without disturbing the entry.
func (t *Tray) SetIcon(icon HICON) {
	if !t.added || icon == 0 {
		return
	}
	data := NOTIFYICONDATAW{
		CbSize: DWORD(unsafe.Sizeof(NOTIFYICONDATAW{})),
		Hwnd:   t.hwnd,
		UID:    t.id,
		UFlags: NIF_ICON,
		HIcon:  icon,
	}
	if ret, _, _ := pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&data))); ret != 0 {
		if t.icon != 0 {
			pDestroyIcon.Call(uintptr(t.icon))
		}
		t.icon = icon
	}
}

// Remove takes the icon back out of the tray.
func (t *Tray) Remove() {
	if !t.added {
		return
	}
	data := NOTIFYICONDATAW{
		CbSize: DWORD(unsafe.Sizeof(NOTIFYICONDATAW{})),
		Hwnd:   t.hwnd,
		UID:    t.id,
	}
	pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&data)))
	t.added = false
	if t.icon != 0 {
		pDestroyIcon.Call(uintptr(t.icon))
		t.icon = 0
	}
}

// MessageBox shows a small modal dialog with an OK button, which is how the app
// answers "what is this thing" from the tray menu.
//
// It is deliberately the shell's own dialog: a tray app that is asked about
// itself should look like it belongs to the machine it is on.
func MessageBox(title, text string) {
	pMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(unsafe.Pointer(utf16Ptr(title))),
		0x00000040, /* MB_ICONINFORMATION */
	)
}

// MenuItem is one line of a popup menu.
type MenuItem struct {
	ID    int
	Label string
	Sep   bool
}

// PopupMenu shows a menu at the cursor and answers with the chosen id, or 0 if
// the user dismissed it. Blocking, like the Win32 call it wraps.
//
// Owner is the window the menu belongs to, and it is not decoration: a popup
// menu whose owner is not the foreground window is dismissed the instant it
// opens, which is why the tray menu of a background app — one that never takes
// focus, and never wants to — so often flashes and disappears. So the owner is
// brought forward for the length of the menu, and the null message Windows
// expects afterwards is posted to hand it back to whoever had it.
func PopupMenu(owner HWND, items []MenuItem) int {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return 0
	}
	defer pDestroyMenu.Call(menu)
	for _, item := range items {
		if item.Sep {
			pAppendMenuW.Call(menu, 0x00000800 /* MF_SEPARATOR */, 0, 0)
			continue
		}
		pAppendMenuW.Call(menu, 0, uintptr(item.ID), uintptr(unsafe.Pointer(utf16Ptr(item.Label))))
	}
	pt := CursorPosition()
	if owner != 0 {
		pSetForegroundWindow.Call(uintptr(owner))
	}
	cmd, _, _ := pTrackPopupMenu.Call(
		menu,
		TPM_RETURNCMD|TPM_RIGHTBUTTON,
		uintptr(pt.X), uintptr(pt.Y),
		0, uintptr(owner), 0,
	)
	if owner != 0 {
		pPostMessageW.Call(uintptr(owner), 0 /* WM_NULL */, 0, 0)
	}
	return int(cmd)
}
