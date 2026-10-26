// Local TechNote export: real UAT screenshots, rendered captions and Vietnamese synthetic audio.
import Foundation
import AVFoundation
import AppKit
import CoreVideo

struct Frame: Decodable { let image: String; let duration: Double }
struct Scene: Decodable { let audio: String; let duration: Double }
struct Manifest: Decodable { let frames: [Frame]; let scenes: [Scene]; let output: String }
let manifest = try JSONDecoder().decode(Manifest.self, from: Data(contentsOf: URL(fileURLWithPath: CommandLine.arguments[1])))
let output = URL(fileURLWithPath: manifest.output)
let silent = output.deletingLastPathComponent().appendingPathComponent("technote-silent.mov")
// These are this generator's own two render outputs, allowing an inspected draft to be regenerated.
for url in [silent, output] where FileManager.default.fileExists(atPath: url.path) {
    try FileManager.default.removeItem(at: url)
}

func renderFrames() throws {
    let writer = try AVAssetWriter(outputURL: silent, fileType: .mov)
    let input = AVAssetWriterInput(mediaType: .video, outputSettings: [
        AVVideoCodecKey: AVVideoCodecType.h264, AVVideoWidthKey: 1280, AVVideoHeightKey: 720,
        AVVideoCompressionPropertiesKey: [AVVideoAverageBitRateKey: 4_000_000],
    ])
    input.expectsMediaDataInRealTime = false
    let attributes: [String: Any] = [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA,
        kCVPixelBufferWidthKey as String: 1280, kCVPixelBufferHeightKey as String: 720,
        kCVPixelBufferCGImageCompatibilityKey as String: true, kCVPixelBufferCGBitmapContextCompatibilityKey as String: true]
    let adaptor = AVAssetWriterInputPixelBufferAdaptor(assetWriterInput: input, sourcePixelBufferAttributes: attributes)
    writer.add(input)
    guard writer.startWriting() else { throw writer.error! }
    writer.startSession(atSourceTime: .zero)
    let fps = 12.0
    var frameIndex: Int64 = 0
    for frame in manifest.frames {
        guard let image = NSImage(contentsOfFile: frame.image)?.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
            throw NSError(domain: "TechNote", code: 1, userInfo: [NSLocalizedDescriptionKey: "Không đọc được ảnh: \(frame.image)"])
        }
        var buffer: CVPixelBuffer?
        CVPixelBufferCreate(kCFAllocatorDefault, 1280, 720, kCVPixelFormatType_32BGRA, attributes as CFDictionary, &buffer)
        let pixel = buffer!
        CVPixelBufferLockBaseAddress(pixel, [])
        let context = CGContext(data: CVPixelBufferGetBaseAddress(pixel), width: 1280, height: 720, bitsPerComponent: 8,
            bytesPerRow: CVPixelBufferGetBytesPerRow(pixel), space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue)!
        context.draw(image, in: CGRect(x: 0, y: 0, width: 1280, height: 720))
        CVPixelBufferUnlockBaseAddress(pixel, [])
        for _ in 0..<max(1, Int((frame.duration * fps).rounded())) {
            while !input.isReadyForMoreMediaData {
                if writer.status == .failed { throw writer.error! }
                Thread.sleep(forTimeInterval: 0.002)
            }
            guard adaptor.append(pixel, withPresentationTime: CMTime(value: frameIndex, timescale: Int32(fps))) else { throw writer.error! }
            frameIndex += 1
        }
    }
    writer.endSession(atSourceTime: CMTime(value: frameIndex, timescale: Int32(fps)))
    input.markAsFinished()
    let done = DispatchSemaphore(value: 0)
    writer.finishWriting { done.signal() }
    done.wait()
    if writer.status != .completed { throw writer.error! }
}
try renderFrames()

Task {
    do {
        let composition = AVMutableComposition()
        let videoAsset = AVURLAsset(url: silent)
        let videoTrack = try await videoAsset.loadTracks(withMediaType: .video)[0]
        let videoDuration = try await videoAsset.load(.duration)
        let destinationVideo = composition.addMutableTrack(withMediaType: .video, preferredTrackID: kCMPersistentTrackID_Invalid)!
        try destinationVideo.insertTimeRange(CMTimeRange(start: .zero, duration: videoDuration), of: videoTrack, at: .zero)
        let destinationAudio = composition.addMutableTrack(withMediaType: .audio, preferredTrackID: kCMPersistentTrackID_Invalid)!
        var at = CMTime.zero
        for scene in manifest.scenes {
            let audioAsset = AVURLAsset(url: URL(fileURLWithPath: scene.audio))
            let tracks = try await audioAsset.loadTracks(withMediaType: .audio)
            let duration = try await audioAsset.load(.duration)
            guard !tracks.isEmpty && duration.seconds > 0 else { throw NSError(domain: "TechNote", code: 2, userInfo: [NSLocalizedDescriptionKey: "Lời đọc rỗng"] ) }
            try destinationAudio.insertTimeRange(CMTimeRange(start: .zero, duration: duration), of: tracks[0], at: at)
            at = CMTimeAdd(at, CMTime(seconds: scene.duration, preferredTimescale: 600))
        }
        let exporter = AVAssetExportSession(asset: composition, presetName: AVAssetExportPreset1280x720)!
        exporter.outputURL = output
        exporter.outputFileType = .mp4
        exporter.shouldOptimizeForNetworkUse = true
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            exporter.exportAsynchronously { continuation.resume() }
        }
        if exporter.status != .completed { throw exporter.error! }
        let finalAsset = AVURLAsset(url: output)
        let duration = try await finalAsset.load(.duration).seconds
        let videos = try await finalAsset.loadTracks(withMediaType: .video)
        let audios = try await finalAsset.loadTracks(withMediaType: .audio)
        let size = try await videos[0].load(.naturalSize)
        let generator = AVAssetImageGenerator(asset: finalAsset)
        generator.appliesPreferredTrackTransform = true
        for (index, second) in [1.0, duration / 2, max(1, duration - 2)].enumerated() {
            let image = try generator.copyCGImage(at: CMTime(seconds: second, preferredTimescale: 600), actualTime: nil)
            let bitmap = NSBitmapImageRep(cgImage: image)
            try bitmap.representation(using: .png, properties: [:])!.write(to: output.deletingLastPathComponent().appendingPathComponent("kiem-tra-video-\(index+1).png"))
        }
        let result: [String: Any] = ["output": output.path, "duration_seconds": duration,
            "video_tracks": videos.count, "audio_tracks": audios.count, "width": size.width, "height": size.height,
            "bytes": (try FileManager.default.attributesOfItem(atPath: output.path)[.size])!]
        print(String(data: try JSONSerialization.data(withJSONObject: result, options: [.prettyPrinted, .sortedKeys]), encoding: .utf8)!)
        exit(0)
    } catch { fputs("TechNote export failed: \(error)\n", stderr); exit(1) }
}
dispatchMain()
