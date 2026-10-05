# Munus TUI demo, recorded with VHS (https://github.com/charmbracelet/vhs).
#
# Regenerate from the repository root. Needs vhs, ttyd, ffmpeg, GNU date and
# the "JetBrainsMono Nerd Font Mono" font (the list headings use Nerd Font
# glyphs). A ./munus from `make build` is used if present, else munus on PATH:
#
#   make build && vhs assets/TUI-demo.tape
#
# The tape points MUNUS_DB_PATH at a new temporary database, so your own tasks
# are never read or changed, and removes it at the end. Deadlines are relative
# to the time of recording.

Output assets/TUI-demo.gif

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 14
Set LetterSpacing 0
Set LineHeight 1.2
Set Width 1400
Set Height 720
Set Padding 20
Set WindowBar Colorful
Set CursorBlink false
Set Framerate 24
Set TypingSpeed 55ms

# Seed a throwaway database (not recorded); stop if it cannot be created.
Hide
Type@1ms `export PATH="$PWD:$PATH" && demo_dir="$(mktemp -d)" && export MUNUS_DB_PATH="$demo_dir/munus.db" || exit 1`
Enter
Type@1ms `munus add -t "Renew TLS certificates" -d "Rotate the internal CA-signed certs on the build hosts." -n "$(date -d yesterday +%F) 17:00" --tag ops >/dev/null`
Enter
Type@1ms `munus add -t "Patch staging servers" -d "Apply kernel and OpenSSL updates, then reboot one host at a time." -n 6h --tag ops >/dev/null`
Enter
Type@1ms `munus add -t "Review import PR" -d "Check the dry-run plan output and the --strict validation errors." -n 1d --tag work >/dev/null`
Enter
Type@1ms `munus add -t "Prep sprint demo" -d "Five-minute walkthrough of the new tag filters." -n 3d --tag work >/dev/null`
Enter
Type@1ms `munus add -t "Draft Q4 roadmap" -d "Collect feature requests and rank them by impact." -n 2w --tag work >/dev/null`
Enter
Type@1ms `munus add -t "Refactor config loader" -d "Split env parsing from file parsing and add table tests." --tag work >/dev/null`
Enter
Type@1ms `munus add -t "Buy groceries" -d "Coffee, eggs, spinach, oat milk." --tag home >/dev/null`
Enter
Type@1ms `munus add -t "Update dependencies" -d "Bump bubbletea and lipgloss; rerun the license check." --tag work >/dev/null && munus complete 8 >/dev/null`
Enter
Type "clear"
Enter
Sleep 500ms
Show

# Running munus with no subcommand opens the new-task form.
Type "munus"
Sleep 400ms
Enter
Sleep 1.5s

Type "Write release notes"
Sleep 400ms
Tab
Type "Summarize the v3.1 changes for the changelog."
Sleep 400ms
Tab
Sleep 1.2s
Type "2d"
Sleep 800ms
Enter
Sleep 1s

# ctrl+l switches to the task list.
Ctrl+L
Sleep 2.5s

# e expands the selected task; s cycles its status (todo -> doing).
Down@400ms 2
Sleep 500ms
Type "e"
Sleep 2s
Type "e"
Sleep 400ms
Type "s"
Sleep 1.8s

# F cycles the status filter: pending -> in progress -> overdue -> done -> all.
Type "F"
Sleep 1s
Type "F"
Sleep 1.8s
Type "F"
Sleep 1.8s
Type "F"
Sleep 1.5s
Type "F"
Sleep 1s

# # cycles the tag filter: home -> ops -> work -> none.
Type "#"
Sleep 1.2s
Type "#"
Sleep 1.2s
Type "#"
Sleep 1.5s
Type "#"
Sleep 1s

# c completes the overdue task.
Type "c"
Sleep 2s

# u edits the selected task: move its deadline to tomorrow.
Down@300ms 4
Sleep 600ms
Type "u"
Sleep 1.2s
Tab@400ms 2
Sleep 600ms
Backspace@40ms 16
Type "1d"
Sleep 800ms
Enter
Sleep 2.2s

# d opens the delete prompt; y confirms.
Down@300ms 5
Sleep 600ms
Type "d"
Sleep 2s
Type "y"
Sleep 2s

# Quit; the CLI reads the same database.
Type "q"
Sleep 800ms
Type "munus list --status doing"
Sleep 400ms
Enter
Sleep 3.5s

Hide
Type `rm -rf -- "${demo_dir:?}"`
Enter
