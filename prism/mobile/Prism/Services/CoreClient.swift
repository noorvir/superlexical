import Foundation

/// Reads lists and pages from Superlexical Core. Core serves the last valid commit.
///
/// Core sends the hash of that commit as the ETag of each response. The client
/// sends the hash back, and Core answers 304 with no body when nothing changed.
/// So a view can read again often at low cost.
enum CoreClient {
    private struct Cached {
        let etag: String
        let data: Data
    }

    private static var cache: [URL: Cached] = [:]

    static func lists() async throws -> [ListSummary] {
        let response: ListsResponse = try await get("lists")
        return response.lists
    }

    static func list(_ slug: String) async throws -> ListDetail {
        try await get("lists/\(slug)")
    }

    static func page(_ path: String) async throws -> Page {
        try await get("page", query: [URLQueryItem(name: "path", value: path)])
    }

    private static func get<Response: Decodable>(_ path: String, query: [URLQueryItem] = []) async throws -> Response {
        var url = ServiceEndpoints.core.appending(path: path)
        if !query.isEmpty {
            url.append(queryItems: query)
        }

        var request = URLRequest(url: url)
        // The cache of this client replaces the cache of URLSession, so that each read asks Core.
        request.cachePolicy = .reloadIgnoringLocalCacheData
        if let cached = cache[url] {
            request.setValue(cached.etag, forHTTPHeaderField: "If-None-Match")
        }

        let (data, response) = try await URLSession.shared.data(for: request)
        let http = response as? HTTPURLResponse

        if http?.statusCode == 304, let cached = cache[url] {
            return try JSONDecoder().decode(Response.self, from: cached.data)
        }

        guard http?.statusCode == 200 else {
            throw ServiceError.badResponse
        }

        if let etag = http?.value(forHTTPHeaderField: "ETag") {
            cache[url] = Cached(etag: etag, data: data)
        }

        return try JSONDecoder().decode(Response.self, from: data)
    }
}

enum ServiceError: LocalizedError {
    case badResponse
    case notConnected
    case rejected(String)

    var errorDescription: String? {
        switch self {
        case .badResponse: "The server sent a response that the app cannot use."
        case .notConnected: "The app is not connected to Ray."
        case .rejected(let message): message
        }
    }
}

struct ListSummary: Decodable, Identifiable, Hashable {
    let slug: String
    let name: String
    let description: String
    let entryCount: Int

    var id: String { slug }
}

struct ListDetail: Decodable {
    let list: ListSummary
    let entries: [EntrySummary]
}

struct EntrySummary: Decodable, Identifiable, Hashable {
    let path: String
    let title: String

    var id: String { path }
}

struct Page: Decodable {
    struct Field: Decodable, Identifiable {
        let name: String
        let value: String

        var id: String { name }
    }

    let path: String
    let title: String
    let fields: [Field]
    let body: String
}

private struct ListsResponse: Decodable {
    let lists: [ListSummary]
}
