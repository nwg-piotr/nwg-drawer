<img src="https://github.com/nwg-piotr/nwg-drawer/assets/20579136/1a7578e8-5332-4e4c-bdce-9b9bf875c0e7" width="90" style="margin-right:10px" align=left alt="logo">
<H1>nwg-drawer</H1><br>

This application is a part of the [nwg-shell](https://nwg-piotr.github.io/nwg-shell) project.

**Nwg-drawer** is an application launcher. It's being developed with [sway](https://github.com/swaywm/sway) and 
[Hyprland](https://github.com/hyprwm/Hyprland) in mind, but should also work with other wlroots-based Wayland 
compositors.

The `nwg-drawer` command displays the application grid. The search entry allows to look for installed applications,
and for files in XDG user directories. The grid view may also be filtered by categories.

You may pin applications by right-clicking them. Pinned items will appear above the application grid. Right-click
a pinned item to unpin it. The pinned items cache is shared with [nwg-menu](https://github.com/nwg-piotr/nwg-menu).

Below the grid there is the **power bar** - a row of buttons to lock the screen, exit the compositor, reboot, suspend 
and power the machine off. For each button to appear, you need to provide a corresponding command. See "Command line 
arguments" below. If the power bar is present, pressing **Tab** will move focus to its first button.

<img src="https://github.com/nwg-piotr/nwg-drawer/assets/20579136/8f4eacb4-5395-4350-889b-a9037aa34f08" width=640 alt="screenshot"><br>

To close the window w/o running a program, you may use the `Esc` key, or right-click the window next to the grid.

## Installation

[![Packaging status](https://repology.org/badge/vertical-allrepos/nwg-drawer.svg)](https://repology.org/project/nwg-drawer/versions)

### Dependencies

- go
- gtk3
- gtk-layer-shell
- xdg-utils

Optional (recommended):

- thunar
- foot

You may use another file manager and terminal emulator (see command line arguments), but mentioned above have been
confirmed to work well with the program. Also see **Files** below.

### Steps

1. Clone the repository, cd into it.
2. Install necessary golang libraries with `make get`.
3. `make build`
4. `sudo make install`

## Command line arguments

```text
Usage of nwg-drawer:
  -c uint
    	number of Columns (default 6)
  -close
    	close drawer of existing instance
  -closebtn string
    	close button position: 'left' or 'right', 'none' by default (default "none")
  -d	Turn on Debug messages
  -fm string
    	File Manager (default "thunar")
  -fscol uint
    	File Search result COLumns (default 2)
  -fslen int
    	File Search name LENgth Limit (default 80)
  -ft
    	Force Theme for libadwaita apps, by adding 'GTK_THEME=<default-gtk-theme>' env var; ignored if wm argument == 'uwsm'
  -g string
    	GTK theme name
  -i string
    	GTK icon theme name
  -is int
    	Icon Size (default 64)
  -k	set GTK layer shell Keyboard interactivity to 'on-demand' mode
  -lang string
    	force lang, e.g. "en", "pl"
  -mb int
    	Margin Bottom
  -ml int
    	Margin Left
  -mr int
    	Margin Right
  -mt int
    	Margin Top
  -nocats
    	Disable filtering by category
  -nofs
    	Disable file search
  -o string
    	name of the Output to display the drawer on (sway & Hyprland only)
  -open
    	open drawer of existing instance
  -ovl
    	use OVerLay layer
  -pbexit string
    	command for the Exit power bar icon
  -pblock string
    	command for the Lock power bar icon
  -pbpoweroff string
    	command for the Poweroff power bar icon
  -pbreboot string
    	command for the Reboot power bar icon
  -pbsize int
    	power bar icon size (only works w/ built-in icons) (default 64)
  -pbsleep string
    	command for the sleep power bar icon
  -pbuseicontheme
    	use icon theme instead of built-in icons in power bar
  -r	Leave the program resident in memory
  -s string
    	Styling: css file name (default "drawer.css")
  -spacing uint
    	icon spacing (default 20)
  -term string
    	Terminal emulator (default "foot")
  -v	display Version information
  -wm string
    	use swaymsg exec (with 'sway' argument) or hyprctl dispatch exec (with 'hyprland') or riverctl spawn (with 'river') or niri msg action spawn -- (with 'niri') or uwsm app -- (with 'uwsm' for Universal Wayland Session Manager) to launch programs
  ```

  *NOTE: the `$TERM` environment variable overrides the `-term` argument.*

### About the `-wm` argument

If you want to run commands through the compositor or through the Universal Wayland Session Manager, use the `-wm` flag.

| Flag value | Will run command with      |
| ---------- |----------------------------|
| sway       | `swaymsg exec`             |
| hyprland   | `hyprctl dispatch exec`    |
| river      | `riverctl spawn`           |
| niri       | `niri msg action spawn --` |
| uwsm       | `uwsm app --`              |

Nwg-drawer will check if it's actually running on the given compositor, or if `uwsm` is installed. If not, it will run 
the command directly. The only exception is `-wm river`, as I have no idea how to confirm it's running.

## Running

You may use the drawer in two ways:

1. Simply run the `nwg-drawer` command, by adding a key binding to your sway config file, e.g.:

```text
bindsym Mod1+F1 exec nwg-drawer
```

2. Run a resident instance on startup, and use the `nwg-drawer` command to show the window, e.g.:

```text
exec_always nwg-drawer -r
bindsym Mod1+F1 exec nwg-drawer
```

The second line does nothing but `pkill -USR1 nwg-drawer`, so you may just use this command instead. Actually
this should be a little bit faster.

Running a resident instance should speed up use of the drawer significantly. Pay attention to the fact, that you
need to `pkill -f nwg-drawer` and reload the compositor to apply any new arguments!

If you want to explicitly specify commands to open and close the resident instance, which can be helpful for touchpad gestures, please use the `-open` and `-close` parameters. Similarly, some signals can also be use: pkill -USR2 nwg-drawer to open and pkill -SIGRTMIN+3 nwg-drawer to close.

For a MacOS-style three-finger pinch:

```text
bindgesture pinch:4:inward exec pkill -SIGUSR2 nwg-drawer
bindgesture pinch:4:outward exec pkill -SIGRTMIN+3 nwg-drawer
```

## Logging

Over the last few years, I've become certain that the program will never be 100% stable, due to the imperfect working 
of GTK3 bindings in golang. Random crashes will always happen. In case you encounter a __repeatable issue__, please attach 
a log to the bug report. If you use the resident instance, you'll see nothing in the terminal. Please edit your sway 
config file:

```text
exec nwg-drawer -r -d 2> ~/drawer.log
```

exit sway, launch it again and include the `drawer.log` content in the GitHub issue. Do not use `exec_always` here: 
it'll destroy the log file content on sway reload.

## Styling

Edit `~/.config/nwg-drawer/drawer.css` to your taste.

## File search

When the search phrase is at least 3 characters long, your XDG user directories are being searched.

Use the **left mouse button** to open a file with the `xdg-open` command. As configuring file associations for it is
PITA, you may override them, by creating the `~/.config/nwg-drawer/preferred-apps.json` file with your own definitions.

### Sample `preferred-apps.json` file content

```json
{
  "\\.pdf$": "atril",
  "\\.svg$": "inkscape",
  "\\.(jpg|png|tiff|gif)$": "swayimg",
  "\\.(mp3|ogg|flac|wav|wma)$": "audacious",
  "\\.(avi|mp4|mkv|mov|wav)$": "mpv",
  "\\.(doc|docx|xls|xlsx)$": "libreoffice"
}
```

Use the **right mouse button** to open the file with your file manager (see `-fm` argument). The result depends
on the file manager you use.

- thunar will open the file location
- pcmanfm will open the file with its associated program
- caja won't open anything, except for directories

I've noy yet tried other file managers.

### File search exclusions

You may want to exclude some paths inside your XDG user directories from searching. If so, define exclusions in the
`~/.config/nwg-drawer/excluded-dirs` file, e.g. like this:

```text
# exclude all paths containing 'node_modules'
node_modules
```

### Calculations in the search box

If the search box is not empty, and you press Enter, the search box content will be evaluated as an arithmetic operation.
If the result is not an error, it will be displayed in a small window, and copied to the clipboard with wl-copy.
Press any key to close the window.

You may change the result label styling e.g. like this:

```css
/* math operation result label */
#math-label {
    font-weight: bold;
    font-size: 16px
}
```

## Credits

This program uses some great libraries:

- [gotk4](https://github.com/diamondburned/gotk4) by [diamondburned](https://github.com/diamondburned) released under [GNU Affero General Public License v3.0](https://github.com/diamondburned/gotk4/blob/4/LICENSE.md)
- [go-sway](https://github.com/joshuarubin/go-sway) Copyright (c) 2019 Joshua Rubin
- [go-singleinstance](github.com/allan-simon/go-singleinstance) Copyright (c) 2015 Allan Simon
- [logrus](https://github.com/sirupsen/logrus) Copyright (c) 2014 Simon Eskildsen
- [fsnotify](https://github.com/fsnotify/fsnotify) Copyright (c) 2012-2019 fsnotify Authors
- [expr](https://github.com/expr-lang/expr) Copyright (c) 2018 Anton Medvedev

## License

nwg-drawer is licensed under the GNU Affero General Public License v3.0 or later (AGPLv3+).  
Copyright (C) 2001-2025 Piotr Miller & Contributors.

You should have received a copy of the GNU Affero General Public License along with this program.  
If not, see [https://www.gnu.org/licenses/agpl-3.0.html](https://www.gnu.org/licenses/agpl-3.0.html).

See the [LICENSE](./LICENSE) file for full license text.
