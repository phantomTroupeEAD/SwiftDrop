# Chapter 9 — Required Figures

## Figure 9.1: Customer 4-Step Parcel Booking Wizard Journey Flowchart
The customer progresses through pickup details, delivery details, parcel/service and quote, and review/confirmation. Validation preserves entered values. Changed or expired quotes require a fresh quote and another review. Only an accepted backend response confirms a booking. An unchanged retry reuses the same idempotency key; edited submissions use a new key.

## Figure 9.2: Public Real-Time Tracking Live View Journey
A visitor looks up a tracking number, views privacy-safe status/timeline information, and receives live change notifications through SSE. Notifications trigger authoritative state refreshes. Disconnects are visibly distinguished from live updates, reconnection triggers a refresh, and unavailable streaming uses bounded polling. Leaving the page closes the stream and timers.

## Placement
Insert Figure 9.1 beside the Chapter 9 booking wizard explanation, and Figure 9.2 beside the public tracking/live-view explanation. Use SVG for sharp vector output, or PNG where required. Open the draw.io file at app.diagrams.net to edit both pages.

## Accuracy limits
The four-step screen grouping is a proposed UX grouping derived from the documented fields; it has not been verified against implemented screens. Exact quote path and field requirements follow M2. SSE event/replay/heartbeat policy and fallback timing require M5 confirmation. These diagrams are design documentation, not evidence of passing implementation tests. The optional FSD architecture / useSSE state-machine suite is not included; current M6 documentation uses feature-based architecture and does not establish an implemented FSD/useSSE contract.
