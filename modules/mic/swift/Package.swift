// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "Mic",
    platforms: [.macOS(.v13), .iOS(.v16)],
    products: [.library(name: "Mic", type: .dynamic, targets: ["Mic"])],
    dependencies: [],
    targets: [
        .target(name: "Mic"),
    ],
    swiftLanguageModes: [.v5]
)
