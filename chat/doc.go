// Package chat is the headless core of a terminal chat agent. A [Chat] owns the
// conversation transcript and drives an ai-toolkit agent, notifying a single
// [Observer] whenever the transcript changes so a front-end can re-render.
//
// The core has no dependency on any UI toolkit and renders nothing itself: it
// stores plain semantic text in [Line] and leaves every glyph, color and layout
// decision to the front-end. The bundled Bubble Tea renderer lives in package
// ui, but it is only one Observer — drive a Chat from a test, a log sink or
// your own UI just as well.
//
// A Chat is safe for concurrent use. Input submitted while a turn is running is
// queued and replayed in order once the turn ends, so callers never have to
// check whether the core is busy before calling [Chat.Submit].
package chat
