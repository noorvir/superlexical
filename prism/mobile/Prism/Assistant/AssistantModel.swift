import Foundation
import Observation

/// State of the search and chat screen.
///
/// Search and chat are two modes of one screen. A search has no session. A
/// switch to chat converts the current search into a session, so the query and
/// its results start the conversation. A switch back to search starts over.
@Observable
final class AssistantModel {
    enum Mode: String, CaseIterable, Identifiable {
        case search = "Search"
        case chat = "Chat"

        var id: Self { self }
    }

    struct SearchResult {
        var query: String
        var text: [TextResult] = []
        var images: [ImageResult] = []
        var summary = ""
        /// Set when Ray has finished the search. A chat session can start from it.
        var searchID: String?
    }

    struct Message: Identifiable {
        enum Role {
            case user, assistant
        }

        let id = UUID()
        let role: Role
        var text: String
    }

    var mode = Mode.search {
        didSet {
            if mode != oldValue {
                modeChanged()
            }
        }
    }

    var input = ""
    private(set) var result: SearchResult?
    private(set) var isSearching = false
    private(set) var suggestion: ListSuggestion?
    private(set) var lists: [ListSummary] = []
    private(set) var messages: [Message] = []
    private(set) var isAnswering = false
    /// Counts the additions that the person started, to trigger feedback.
    private(set) var additionCount = 0
    var errorMessage: String?

    private let acp = ACPClient()
    private var sessionID: String?
    private var searchTask: Task<Void, Never>?

    /// The session of chats that do not start from a search. It persists between launches.
    private var mainSessionID: String? {
        get { UserDefaults.standard.string(forKey: "mainSessionID") }
        set { UserDefaults.standard.set(newValue, forKey: "mainSessionID") }
    }

    init() {
        acp.onUpdate = { [weak self] update in
            self?.apply(update)
        }
    }

    func submit() {
        let text = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else {
            return
        }

        switch mode {
        case .search:
            search(text)
        case .chat:
            input = ""
            Task { await send(text) }
        }
    }

    func loadLists() async {
        do {
            lists = try await CoreClient.lists()
        } catch {
            lists = []
        }
    }

    /// Adds the current search topic to a list. The work runs in the background on Ray.
    func add(to list: String) {
        guard let title = suggestion?.title ?? result?.text.first?.title ?? result?.query else {
            return
        }

        suggestion = nil
        Task {
            do {
                try await RayClient.startAddition(list: list, title: title)
                additionCount += 1
            } catch {
                errorMessage = error.localizedDescription
            }
        }
    }

    func dismissSuggestion() {
        suggestion = nil
    }

    private func search(_ query: String) {
        searchTask?.cancel()
        result = SearchResult(query: query)
        suggestion = nil
        isSearching = true

        searchTask = Task {
            do {
                for try await event in RayClient.search(query) {
                    switch event {
                    case .result(let text, let images):
                        result?.text = text
                        result?.images = images
                    case .summary(let delta):
                        result?.summary += delta
                    case .suggestion(let suggested):
                        suggestion = suggested
                    case .done(let searchID):
                        result?.searchID = searchID
                    case .error(let message):
                        errorMessage = message
                    }
                }
            } catch is CancellationError {
                return
            } catch {
                errorMessage = error.localizedDescription
            }

            if !Task.isCancelled {
                isSearching = false
            }
        }
    }

    private func modeChanged() {
        input = ""
        errorMessage = nil

        switch mode {
        case .chat:
            Task { await startChat() }
        case .search:
            searchTask?.cancel()
            isSearching = false
            result = nil
            suggestion = nil
            messages = []
            sessionID = nil
        }
    }

    private func startChat() async {
        messages = []
        sessionID = nil

        do {
            try await acp.connect()

            if let searchID = result?.searchID {
                let id = try await RayClient.createSession(searchID: searchID)
                try await acp.loadSession(id)
                // The search card shows the start of this session, so the replayed messages are not shown again.
                messages = []
                sessionID = id
                return
            }

            if let id = mainSessionID, (try? await acp.loadSession(id)) != nil {
                sessionID = id
                return
            }

            messages = []
            let id = try await acp.newSession()
            mainSessionID = id
            sessionID = id
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func send(_ text: String) async {
        guard let sessionID else {
            errorMessage = ServiceError.notConnected.localizedDescription
            return
        }

        messages.append(Message(role: .user, text: text))
        isAnswering = true

        do {
            try await acp.prompt(text, sessionID: sessionID)
        } catch {
            errorMessage = error.localizedDescription
        }

        isAnswering = false
    }

    private func apply(_ update: ACPClient.Update) {
        switch update {
        case .userText(let text):
            messages.append(Message(role: .user, text: text))
        case .agentText(let text):
            if let last = messages.indices.last, messages[last].role == .assistant {
                messages[last].text += text
            } else {
                messages.append(Message(role: .assistant, text: text))
            }
        case .toolStarted:
            break
        }
    }
}
