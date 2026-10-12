import Foundation
import Observation

enum ServiceState {
    case checking
    case running
    case unreachable
}

@Observable
final class ServiceMonitor {
    private(set) var core = ServiceState.checking
    private(set) var ray = ServiceState.checking

    func refresh() async {
        async let coreState = Self.state(of: ServiceEndpoints.core)
        async let rayState = Self.state(of: ServiceEndpoints.ray)

        core = await coreState
        ray = await rayState
    }

    private nonisolated static func state(of service: URL) async -> ServiceState {
        var request = URLRequest(url: service.appending(path: "health"))
        request.timeoutInterval = 5

        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            let http = response as? HTTPURLResponse

            if http?.statusCode == 200 {
                return .running
            }

            return .unreachable
        } catch {
            return .unreachable
        }
    }
}
