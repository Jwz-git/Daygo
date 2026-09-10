import Darwin
import Foundation

func captureDiagnostic(_ stage: String) {
    guard ProcessInfo.processInfo.environment["DAYGO_CAPTURE_DEBUG"] == "1" else {
        return
    }
    let uptime = ProcessInfo.processInfo.systemUptime
    let line = String(format: "[daygo.capture] uptime=%.3f stage=%@\n", uptime, stage)
    line.withCString { bytes in
        _ = Darwin.write(STDERR_FILENO, bytes, strlen(bytes))
    }
}
