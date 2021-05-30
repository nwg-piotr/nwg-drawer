# nwg-drawer

This application is a part of the [nwg-shell](https://github.com/nwg-piotr/nwg-shell) project.

Application drawer for sway Wayland compositor, and a golang replacement to the `nwggrid` command (a part of
[nwg-launchers](https://github.com/nwg-piotr/nwg-launchers)). This program is being developed with
[sway](https://github.com/swaywm/sway) in mind. It should also work with other wlroots-based Wayland compositors.

Old features: application grid, pinned apps, search entry. New features: gtk-layer-shell support, filtering by categories, searching files in XDG user dirs.

![screenshot-01.png](https://scrot.cloud/images/2021/05/30/screenshot-01.png)

[more screenshots](https://scrot.cloud/album/nwg-drawer.Bogd)

The `nwg-drawer` command displays the application grid. The search entry allows to look for installed applications, and for files in XDG user directories. The application grid may also be filtered by categories.

You may pin applications by right-clicking them. Pinned items will appear above the application grid. Right-click
a pinned item to unpin it. The pinned items cache is shared with `nwg-menu` and `nwggrid`.
