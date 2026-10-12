import Foundation

/// Calls the HTTP endpoints of Ray for search and list additions. Chat uses `ACPClient`.
enum RayClient {
    /// Runs one search. The stream yields the parts of the result as Ray makes them.
    static func search(_ query: String) -> AsyncThrowingStream<SearchEvent, Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    var request = URLRequest(url: ServiceEndpoints.ray.appending(path: "search"))
                    request.httpMethod = "POST"
                    request.setValue("application/json", forHTTPHeaderField: "Content-Type")
                    request.httpBody = try JSONEncoder().encode(["query": query])

                    let (bytes, response) = try await URLSession.shared.bytes(for: request)
                    guard (response as? HTTPURLResponse)?.statusCode == 200 else {
                        throw ServiceError.badResponse
                    }

                    for try await line in bytes.lines {
                        guard line.hasPrefix("data: ") else {
                            continue
                        }

                        let data = Data(line.dropFirst(6).utf8)
                        let event = try JSONDecoder().decode(SearchEvent.self, from: data)
                        continuation.yield(event)
                    }

                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }

            continuation.onTermination = { _ in task.cancel() }
        }
    }

    /// Converts a finished search into a chat session and returns the session ID.
    static func createSession(searchID: String) async throws -> String {
        let url = ServiceEndpoints.ray.appending(path: "searches/\(searchID)/session")
        let response: SessionResponse = try await post(url, body: [String: String]())
        return response.sessionId
    }

    /// Starts a background agent that adds an entry to a list.
    static func startAddition(list: String, title: String) async throws {
        let url = ServiceEndpoints.ray.appending(path: "additions")
        let _: AdditionStarted = try await post(url, body: ["list": list, "title": title])
    }

    static func additions() async throws -> [Addition] {
        let url = ServiceEndpoints.ray.appending(path: "additions")
        let (data, response) = try await URLSession.shared.data(from: url)
        guard (response as? HTTPURLResponse)?.statusCode == 200 else {
            throw ServiceError.badResponse
        }

        return try JSONDecoder().decode(AdditionsResponse.self, from: data).additions
    }

    private static func post<Body: Encodable, Response: Decodable>(_ url: URL, body: Body) async throws -> Response {
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(body)

        let (data, response) = try await URLSession.shared.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard (200..<300).contains(status) else {
            let failure = try? JSONDecoder().decode(Failure.self, from: data)
            throw ServiceError.rejected(failure?.message ?? "Ray did not accept the request.")
        }

        return try JSONDecoder().decode(Response.self, from: data)
    }
}

enum SearchEvent: Decodable {
    case result(text: [TextResult], images: [ImageResult])
    case summary(delta: String)
    case suggestion(ListSuggestion)
    case done(searchID: String)
    case error(message: String)

    private enum CodingKeys: String, CodingKey {
        case type, text, images, delta, list, title, searchId, message
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let type = try container.decode(String.self, forKey: .type)

        switch type {
        case "result":
            let text = try container.decode([TextResult].self, forKey: .text)
            let images = try container.decode([ImageResult].self, forKey: .images)
            self = .result(text: text, images: images)
        case "summary":
            self = .summary(delta: try container.decode(String.self, forKey: .delta))
        case "suggestion":
            let list = try container.decode(ListSuggestion.SuggestedList.self, forKey: .list)
            let title = try container.decode(String.self, forKey: .title)
            self = .suggestion(ListSuggestion(list: list, title: title))
        case "done":
            self = .done(searchID: try container.decode(String.self, forKey: .searchId))
        case "error":
            self = .error(message: try container.decode(String.self, forKey: .message))
        default:
            throw DecodingError.dataCorruptedError(forKey: .type, in: container, debugDescription: "Unknown search event \(type)")
        }
    }
}

struct TextResult: Decodable {
    let title: String
    let text: String
    let url: URL
    let source: String
}

struct ImageResult: Decodable, Identifiable {
    let url: URL
    let thumbnailUrl: URL
    let pageUrl: URL
    let title: String

    var id: URL { url }
}

struct ListSuggestion {
    struct SuggestedList: Decodable {
        let slug: String
        let name: String
    }

    let list: SuggestedList
    /// Title of the entry that the suggestion adds.
    let title: String
}

struct Addition: Decodable, Identifiable {
    enum Status: String, Decodable {
        case queued, running, done, failed
    }

    let id: String
    let list: String
    let title: String
    let path: String
    let status: Status
}

private struct SessionResponse: Decodable {
    let sessionId: String
}

private struct AdditionStarted: Decodable {
    let id: String
}

private struct AdditionsResponse: Decodable {
    let additions: [Addition]
}

private struct Failure: Decodable {
    let message: String
}
