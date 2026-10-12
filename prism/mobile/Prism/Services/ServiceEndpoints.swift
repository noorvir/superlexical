import Foundation

/// Base URLs of the Superlexical services.
///
/// The defaults address services on the Mac that runs the simulator. A build
/// overrides them with `PRISM_CORE_URL` and `PRISM_RAY_URL` in `Local.xcconfig`.
enum ServiceEndpoints {
    static let core = configuredURL(key: "PRISM_CORE_URL", defaultValue: "http://127.0.0.1:4100")
    static let ray = configuredURL(key: "PRISM_RAY_URL", defaultValue: "http://127.0.0.1:4200")

    private static func configuredURL(key: String, defaultValue: String) -> URL {
        let configured = Bundle.main.object(forInfoDictionaryKey: key) as? String

        if let configured, let url = URL(string: configured), url.host() != nil {
            return url
        }

        return URL(string: defaultValue)!
    }
}
