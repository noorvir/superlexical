import Foundation

/// A client for the chat sessions of Ray over the Agent Client Protocol.
///
/// The client sends JSON-RPC messages on one WebSocket. It supports the
/// methods that Prism uses: `initialize`, `session/new`, `session/load`, and
/// `session/prompt`. Session updates arrive through `onUpdate`.
final class ACPClient {
    enum Update {
        case userText(String)
        case agentText(String)
        case toolStarted
    }

    var onUpdate: ((Update) -> Void)?

    private var socket: URLSessionWebSocketTask?
    private var nextID = 1
    /// Waiting requests by ID. Each one receives the JSON of its result.
    private var pending: [Int: CheckedContinuation<Data, Error>] = [:]

    func connect() async throws {
        if socket != nil {
            return
        }

        guard var components = URLComponents(url: ServiceEndpoints.ray.appending(path: "acp"), resolvingAgainstBaseURL: false) else {
            throw ServiceError.notConnected
        }
        components.scheme = components.scheme == "https" ? "wss" : "ws"
        guard let url = components.url else {
            throw ServiceError.notConnected
        }

        let socket = URLSession.shared.webSocketTask(with: url)
        self.socket = socket
        socket.resume()
        Task { await receive(from: socket) }

        _ = try await request("initialize", ["protocolVersion": 1, "clientCapabilities": [String: Any]()])
    }

    func newSession() async throws -> String {
        let result = try await request("session/new", ["cwd": "/", "mcpServers": [Any]()])
        guard let sessionID = result["sessionId"] as? String else {
            throw ServiceError.badResponse
        }

        return sessionID
    }

    /// Attaches to an existing session. Ray sends the earlier messages through `onUpdate` before this call returns.
    func loadSession(_ sessionID: String) async throws {
        _ = try await request("session/load", ["sessionId": sessionID, "cwd": "/", "mcpServers": [Any]()])
    }

    /// Sends one message and returns when Ray has finished its answer.
    func prompt(_ text: String, sessionID: String) async throws {
        let block: [String: Any] = ["type": "text", "text": text]
        _ = try await request("session/prompt", ["sessionId": sessionID, "prompt": [block]])
    }

    func disconnect() {
        socket?.cancel(with: .goingAway, reason: nil)
        socket = nil
        failPending(with: ServiceError.notConnected)
    }

    private func request(_ method: String, _ params: [String: Any]) async throws -> [String: Any] {
        guard let socket else {
            throw ServiceError.notConnected
        }

        let id = nextID
        nextID += 1
        let message: [String: Any] = ["jsonrpc": "2.0", "id": id, "method": method, "params": params]
        let data = try JSONSerialization.data(withJSONObject: message)
        let text = String(decoding: data, as: UTF8.self)

        let result = try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Data, Error>) in
            pending[id] = continuation
            socket.send(.string(text)) { error in
                guard let error else {
                    return
                }

                Task { @MainActor in
                    self.pending.removeValue(forKey: id)?.resume(throwing: error)
                }
            }
        }

        return try JSONSerialization.jsonObject(with: result) as? [String: Any] ?? [:]
    }

    private func receive(from socket: URLSessionWebSocketTask) async {
        while true {
            do {
                let message = try await socket.receive()
                guard case .string(let text) = message else {
                    continue
                }

                handle(text)
            } catch {
                if self.socket === socket {
                    self.socket = nil
                }

                failPending(with: error)
                return
            }
        }
    }

    private func handle(_ text: String) {
        guard let message = try? JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any] else {
            return
        }

        if let id = message["id"] as? Int, let continuation = pending.removeValue(forKey: id) {
            if let error = message["error"] as? [String: Any] {
                let description = error["message"] as? String ?? "Ray reported an error."
                continuation.resume(throwing: ServiceError.rejected(description))
            } else {
                let result = message["result"] as? [String: Any] ?? [:]
                let data = (try? JSONSerialization.data(withJSONObject: result)) ?? Data("{}".utf8)
                continuation.resume(returning: data)
            }

            return
        }

        guard message["method"] as? String == "session/update",
              let params = message["params"] as? [String: Any],
              let update = params["update"] as? [String: Any],
              let kind = update["sessionUpdate"] as? String
        else {
            return
        }

        let content = update["content"] as? [String: Any]
        let chunk = content?["text"] as? String

        switch kind {
        case "agent_message_chunk":
            if let chunk {
                onUpdate?(.agentText(chunk))
            }
        case "user_message_chunk":
            if let chunk {
                onUpdate?(.userText(chunk))
            }
        case "tool_call":
            onUpdate?(.toolStarted)
        default:
            break
        }
    }

    private func failPending(with error: Error) {
        let waiting = pending
        pending = [:]
        for continuation in waiting.values {
            continuation.resume(throwing: error)
        }
    }
}
