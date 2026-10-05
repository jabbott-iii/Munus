# Munus CLI demo, recorded with VHS (https://github.com/charmbracelet/vhs).
#
# Regenerate from the repository root. Needs vhs, ttyd, ffmpeg, GNU date and
# the "JetBrains Mono" and "DejaVu Sans Mono" fonts. A ./munus from
# `make build` is used if present, else munus on PATH:
#
#   make build && vhs assets/CLI-demo.tape
#
# The tape runs in a new temporary directory with HOME and MUNUS_DB_PATH
# pointing into it, so the database, the export and the import backup never
# touch your own tasks or home directory; the directory is removed at the end.
# Relative deadlines count from the time of recording; bump the absolute date
# in the "add" scene once it has passed.
#
# VHS builds the GIF palette from every frame held in memory (about 6 GB for
# this ~90 s tape). On a smaller machine, point Output at a directory
# (Output "frames/") and encode the PNG frames with a two-pass
# ffmpeg palettegen/paletteuse instead.

Output assets/CLI-demo.gif

Set Shell "bash"
Set FontFamily "JetBrains Mono,DejaVu Sans Mono"
Set FontSize 14
Set LetterSpacing 0
Set LineHeight 1.2
Set Width 1100
Set Height 760
Set Padding 20
Set WindowBar Colorful
Set CursorBlink false
Set Framerate 24
Set TypingSpeed 30ms

# Work in a throwaway directory (not recorded); stop if it cannot be created.
Hide
Type@1ms `export PATH="$PWD:$PATH" && demo_dir="$(mktemp -d)" && cd "$demo_dir" && export HOME="$demo_dir" MUNUS_DB_PATH="$demo_dir/tasks.db" && clear || exit 1`
Enter
Sleep 500ms
Show

# --- Version and help -------------------------------------------------------
Type "munus --version"
Enter
Sleep 800ms
Type "munus --help"
Enter
Sleep 3s
Type "# Optional: choose the database file (default: your per-user data dir)"
Enter
Type `export MUNUS_DB_PATH="$PWD/tasks.db"`
Enter
Sleep 1.5s

# An existing, overdue task for the list and filter scenes (not recorded).
Hide
Type@1ms `munus add -t "Renew TLS certificates" -d "Rotate certs on the build hosts" -n "$(date -d yesterday +%F) 17:00" --tag ops >/dev/null && clear`
Enter
Sleep 500ms
Show

# --- add ----------------------------------------------------------------------
Type "# Add tasks with relative or absolute deadlines and tags"
Enter
Type `munus add -t "Patch staging servers" -d "Kernel and OpenSSL updates" -n 6h --tag ops`
Enter
Sleep 800ms
Type `munus add -t "Review import PR" -d "Check the dry-run plan output" -n "1d 2h" --tag work`
Enter
Sleep 800ms
Type `munus add -t "Ship v3.1" -d "Tag, build and publish the release" -n "2026-12-01 09:00" --tag work,release`
Enter
Sleep 2.5s

# --- list ---------------------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# List and filter (--overdue, --status, --tag, --pending, --completed); filters combine"
Enter
Type "munus list --overdue"
Enter
Sleep 2.5s
Type "munus list --status todo --tag work"
Enter
Sleep 3.5s

# --- edit ---------------------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# Edit only the fields you pass"
Enter
Type "munus edit 3 --status doing --deadline 2d"
Enter
Sleep 700ms
Type `munus edit 4 --title "Ship v3.1.0" --clear-deadline --tag urgent --untag release`
Enter
Sleep 700ms
Type "munus list --tag work"
Enter
Sleep 4s

# --- complete -----------------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# Complete tasks, or reopen them with --undo"
Enter
Type "munus complete 2"
Enter
Sleep 600ms
Type "munus complete 1"
Enter
Sleep 600ms
Type "munus complete 1 --undo"
Enter
Sleep 600ms
Type "munus list --completed"
Enter
Sleep 3s

# --- export -------------------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# Export to versioned JSON: dry run, file or standard output"
Enter
Type "munus export --dry-run"
Enter
Sleep 1s
Type "munus export --file backup.json"
Enter
Sleep 1s
Type "munus export --stdout --pending-only | head -n 17"
Enter
Sleep 3.5s

# --- delete + import ----------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# Delete asks first; import restores from the export, keeping IDs"
Enter
Type "munus delete 3"
Enter
Sleep 1s
Type "y"
Enter
Sleep 1s
Type "munus import --file backup.json --dry-run"
Enter
Sleep 2.5s
Type "munus import --file backup.json --backup"
Enter
Sleep 3.5s

# --- import from standard input ----------------------------------------------
Hide
Type "clear"
Enter
Show
Type "# Pipe an export into import; --strict rejects anything suspicious"
Enter
Type "munus export --stdout | munus import --file - --dry-run --strict"
Enter
Sleep 3s

# --- final state ----------------------------------------------------------------
Hide
Type "clear"
Enter
Show
Type "munus list"
Enter
Sleep 5s

Hide
Type `cd / && rm -rf -- "${demo_dir:?}"`
Enter
