package app

import "github.com/wailsapp/wails/v3/pkg/application"

// MacMenu returns the macOS application menu: only the standard App, Edit and
// Window menus. App actions live in the frontend command registry
// (frontend/src/lib/commands) so their keybindings stay remappable; a native
// menu item would claim its accelerator before the webview sees the key. Wails'
// default File and View menus are left out because they bind Cmd+W to closing
// the window and Cmd+R to reloading the webview, which would throw away open
// tabs and pending edits.
//
// Windows needs no menu: windows show none by default, and WebView2's browser
// accelerator keys are disabled by Wails.
func MacMenu() *application.Menu {
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	menu.AddRole(application.EditMenu) // Cut, Copy, Paste and Select All in text fields
	menu.AddRole(application.WindowMenu)
	return menu
}
