Tuick-Tack for macOS (experimental)

Install
  1. Move Tuick-Tack.app to ~/Applications (recommended, no admin) or to
     /Applications.
  2. Open it. Gatekeeper warns because this build is ad-hoc signed and not
     notarized. Right-click the app and choose Open, or run:
       xattr -dr com.apple.quarantine Tuick-Tack.app
  3. The app starts the background daemon and shows its icon in the menu bar.
     Choose Open in that menu to start the terminal UI.

Data
  Settings and statistics   ~/Library/Application Support/tuick-tack
  Cache, logs and sounds    ~/Library/Caches/tuick-tack
  Login item                ~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist

Notes
  - Keep the app in a stable location. If you move it, the login item path is
    rewritten on the next launch.
  - There is no installer or automatic update yet. To upgrade, quit the menu bar
    app, replace Tuick-Tack.app and open the new copy.
  - The app is ad-hoc signed (no Developer ID) and not notarized; macOS 13 or
    later, Apple silicon and Intel are supported by the universal build.
