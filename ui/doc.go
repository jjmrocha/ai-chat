// Package ui is a Bubble Tea front-end for a chat.Chat.
//
// [Run] is the whole public surface: hand it a core and it takes over the
// terminal until the user quits. It is one Observer of the core, not part of
// it — replace this package with your own renderer without touching your agent
// logic.
//
// The UI renders inline rather than taking over the screen. Finished transcript
// lines are printed above a small live region holding the thinking row, title
// bar, input and status line, so the conversation lands in the terminal's own
// scrollback and selection, copying and scrolling stay the terminal's. The
// trade-off is that printed lines are never repainted, so resizing leaves
// earlier markdown wrapped at the old width.
//
// Keys: Enter sends, Shift+Enter (or Alt+Enter, Ctrl+J) inserts a newline, Up
// and Down walk prompt history, Esc cancels the running turn or command and
// drops queued prompts, Ctrl+C quits.
//
// Colors come from one fixed palette that cannot be switched. Every value is
// either an ANSI index, which the terminal's own profile defines, or empty,
// meaning the terminal's default text color — a fixed set of hex colors can
// only be right for the background it was tuned against, and neither this
// package nor the program embedding it can know the user's. Markdown replies
// follow the same rule: glamour's dark layout with every color taken from the
// palette, and fenced code highlighted in the 16 basic ANSI colors.
package ui
