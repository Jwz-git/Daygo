// Package platform is the port boundary between Go Core and the host operating
// system. It declares interfaces and value types only — the implementation-
// independent half of docs/05 §5.7 and docs/06 — and holds NO implementation.
//
// The rules this package exists to enforce:
//
//   - Go Core stays buildable and testable with CGO_ENABLED=0 on Linux. Nothing
//     here may import Wails, a macOS framework, or any adapter implementation.
//   - Business code never knows whether the adapter is in-process, out-of-process
//     or a native host — it holds a Capture/Media/System/Secrets/Updater and
//     nothing more.
//   - The implementation FORM is undecided (docs/09 §9.8 item 1, due M1). Only
//     implementation-independent semantics belong in this contract; provisional
//     native UI and updater shapes stay minimal until their decisions land.
//
// Two implementations are planned behind these ports and must pass the SAME
// contract suite (platformtest.Suite): internal/platform/fake (every OS) and
// the real darwin adapter (macOS CI only, after the M1 decision). Neither
// exists yet — both are M1 deliverables. An interface the fake passes but the
// real adapter has not run against is not considered verified — docs/06 §6.5.
package platform
