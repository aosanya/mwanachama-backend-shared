// Package mcpui provides a shared visual design system for the hand-rolled
// HTML dashboard Views that backend repos expose over MCP Apps (SEP-1865).
// Each repo still owns its own view's markup and behavior (there is no
// shared component runtime); this package only supplies the CSS so those
// views look consistent with each other and with mwanachama-wakala-studio,
// instead of every repo hand-copying and slowly drifting from its own
// snapshot of the palette.
package mcpui

import _ "embed"

// ThemeCSS is the shared token/base-element stylesheet — see theme.css for
// what it covers. Callers splice it into their own document as a <style>
// block (typically right after the <meta charset> tag), ahead of any
// page-specific <style> block so page rules can still override it via the
// normal CSS cascade.
//
//go:embed theme.css
var ThemeCSS string
