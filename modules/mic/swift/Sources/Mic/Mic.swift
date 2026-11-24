import AVFoundation
import Foundation

// The microphone on Apple's platforms: AVAudioEngine's input, converted to mono
// 16-bit PCM at 16 kHz and yielded in chunks of about 100 ms. The capture runs
// only while the stream is read: ending the stream (the Go side cancels its
// context) stops the engine and releases the microphone.
//
// macOS asks for the microphone on first use and needs
// NSMicrophoneUsageDescription in the app's Info.plist; iOS needs it too, and an
// audio session in the record category.

func makeRecorder() throws -> any Recorder { NativeRecorder() }

enum CaptureError: LocalizedError {
    case denied
    case noDevice
    case failed(String)

    var errorDescription: String? {
        switch self {
        case .denied:
            return "microphone access was not allowed (turn it on in the system's privacy settings)"
        case .noDevice:
            return "no microphone was found"
        case .failed(let why):
            return why
        }
    }
}

final class NativeRecorder: Recorder {
    private static let sampleRate = 16000.0

    func samples() -> AsyncThrowingStream<Data, Error> {
        AsyncThrowingStream { continuation in
            let session = Capture(continuation: continuation)
            continuation.onTermination = { _ in session.stop() }
            Task { await session.start() }
        }
    }
}

/// One capture: permission, the engine, the converter.
private final class Capture: @unchecked Sendable {
    private let continuation: AsyncThrowingStream<Data, Error>.Continuation
    private let engine = AVAudioEngine()
    private let lock = NSLock()
    private var stopped = false

    init(continuation: AsyncThrowingStream<Data, Error>.Continuation) {
        self.continuation = continuation
    }

    func start() async {
        guard await Capture.allowed() else {
            continuation.finish(throwing: CaptureError.denied)
            return
        }
        do {
            #if os(iOS)
            let audio = AVAudioSession.sharedInstance()
            try audio.setCategory(.playAndRecord, mode: .default, options: [.defaultToSpeaker, .allowBluetooth])
            try audio.setActive(true)
            #endif
            try run()
        } catch {
            continuation.finish(throwing: CaptureError.failed("could not start the microphone: \(error.localizedDescription)"))
        }
    }

    private func run() throws {
        let input = engine.inputNode
        let from = input.outputFormat(forBus: 0)
        guard from.sampleRate > 0, from.channelCount > 0 else { throw CaptureError.noDevice }
        guard let to = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true),
              let converter = AVAudioConverter(from: from, to: to) else {
            throw CaptureError.failed("the microphone's format cannot be converted")
        }
        // About 100 ms per callback at the input's rate.
        let frames = AVAudioFrameCount(from.sampleRate / 10)
        input.installTap(onBus: 0, bufferSize: frames, format: from) { [weak self] buffer, _ in
            guard let self = self else { return }
            let capacity = AVAudioFrameCount(Double(buffer.frameLength) * 16000 / from.sampleRate) + 16
            guard let out = AVAudioPCMBuffer(pcmFormat: to, frameCapacity: capacity) else { return }
            var supplied = false
            var error: NSError?
            converter.convert(to: out, error: &error) { _, status in
                if supplied {
                    status.pointee = .noDataNow
                    return nil
                }
                supplied = true
                status.pointee = .haveData
                return buffer
            }
            guard error == nil, out.frameLength > 0, let channel = out.int16ChannelData else { return }
            let data = Data(bytes: channel[0], count: Int(out.frameLength) * MemoryLayout<Int16>.size)
            self.lock.lock()
            let live = !self.stopped
            self.lock.unlock()
            if live { self.continuation.yield(data) }
        }
        engine.prepare()
        try engine.start()
    }

    func stop() {
        lock.lock()
        let already = stopped
        stopped = true
        lock.unlock()
        if already { return }
        engine.inputNode.removeTap(onBus: 0)
        engine.stop()
        #if os(iOS)
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        #endif
    }

    /// Whether the app may record, asking the user the first time.
    private static func allowed() async -> Bool {
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized:
            return true
        case .notDetermined:
            return await AVCaptureDevice.requestAccess(for: .audio)
        default:
            return false
        }
    }
}
