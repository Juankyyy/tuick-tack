package daemon

// supportsWindowPlacement is false on macOS: the app does not move or resize
// the terminal window, and the handshake omits the window object so the TUI
// never tries to send a placement.
const supportsWindowPlacement = false
