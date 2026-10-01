#!/bin/sh
# Starts Disk Clone as root through pkexec (polkit action org.diskclone.run).
# pkexec clears the environment, so the variables needed to open a window
# are passed as arguments to the root-side helper diskclone-root.

# Some Wayland compositors refuse windows from root clients: fall back to
# XWayland and allow root to use the X display of the current user.
# DISKCLONE_WAYLAND=1 keeps native Wayland.
if [ -n "$WAYLAND_DISPLAY" ] && [ "${DISKCLONE_WAYLAND:-0}" != "1" ]; then
    GDK_BACKEND=x11
    command -v xhost >/dev/null 2>&1 && xhost +SI:localuser:root >/dev/null 2>&1
fi

exec pkexec /usr/local/libexec/diskclone-root \
    "$DISPLAY" "${XAUTHORITY:-$HOME/.Xauthority}" "$WAYLAND_DISPLAY" \
    "$XDG_RUNTIME_DIR" "${GDK_BACKEND:-}" "${LANG:-C.UTF-8}" "${DISKCLONE_DEV_SAFE:-}"
