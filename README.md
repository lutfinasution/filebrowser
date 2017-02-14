# filebrowser
Filebrowser + thumbnail viewer
Expanded example based on lxn's excellent walk win32 libraries https://github.com/lxn/walk filebrowser example, adding many new
features.

![screenshot](https://github.com/lutfinasution/filebrowser/blob/master/image/filebrowser01.png?raw=true "screenshot")

Why?
I'm trying to explore golang's capabilities and performance in developing a multithreaded and memory intensive windows desktop app.
For inspiration and comparison, I used my old application, Antares12 which was built using Delphi2007.
So far golang didn't disappoint!

![screenshot](https://github.com/lutfinasution/filebrowser/blob/master/image/filebrowser02.png?raw=true "screenshot")

Features:
  - Image browser, displaying thumbnail of images in a grid.
  - Image http server, serving thumbnail of images in a folder. Use http://localhost:8080/users/photos while the app is running.
  - Very fast and efficient multi threaded thumbnail processing with goroutines and channels.
  - Resizeable thumbnail size.
  - Filesystem/directory monitoring for changes (new/delete/renamed/modified files)

Golang's features explored here:
  - Goroutine, channels and synchronizations.
  - Maps, slices
  - cgo interfacing with external c libs
  - Database interface with sqlite3
  - Image processing (drawings/transforms)

Additional excellent golang libraries used here:
  - https://github.com/pixiv/go-libjpeg/jpeg
  - https://github.com/mattn/go-sqlite3
  - https://github.com/fsnotify/fsnotify
  - https://github.com/anthonynsimon/bild/transform
  
Status:
 - Go language constructs comfortably matched object pascal (delphi) constructs.
 - Many low level routines can easily be converted to golang.
 - Goroutines and channels are truly handy in handling concurrent tasks.
 - Performance in performing cpu and memory intensive operations is excellent. Comparable and sometimes better than delphi.
 - Memory comsumption is larger than delphi/32bit, but not by much. No optimization on my part yet.
 - So far so good...


