import SwiftUI

/// The entries of one list. Entries that Ray is still adding show a progress state.
struct ListDetailView: View {
    let list: ListSummary

    @State private var entries: [EntrySummary] = []
    @State private var additions: [Addition] = []
    @State private var isAddPresented = false
    @State private var newTitle = ""
    @State private var errorMessage: String?

    var body: some View {
        List {
            if !pending.isEmpty {
                Section {
                    ForEach(pending) { addition in
                        LabeledContent(addition.title) {
                            switch addition.status {
                            case .failed:
                                Label("Not Added", systemImage: "exclamationmark.circle")
                                    .foregroundStyle(.red)
                            default:
                                ProgressView()
                                    .accessibilityLabel("Adding")
                            }
                        }
                    }
                }
            }

            Section {
                ForEach(entries) { entry in
                    NavigationLink(entry.title, value: PageRoute(path: entry.path))
                }
            }
        }
        .navigationTitle(list.name)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Add Entry", systemImage: "plus") {
                    newTitle = ""
                    isAddPresented = true
                }
            }
        }
        .alert("Add to \(list.name)", isPresented: $isAddPresented) {
            TextField("Title", text: $newTitle)
            Button("Cancel", role: .cancel) {}
            Button("Add") {
                add(newTitle)
            }
        } message: {
            Text("Ray writes the entry and links it to your other pages.")
        }
        .alert("Cannot Add the Entry", isPresented: errorIsPresented) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(errorMessage ?? "")
        }
        .refreshable {
            await load()
        }
        .task {
            // Ray adds entries in the background, so the list is read again until no addition is in progress.
            while !Task.isCancelled {
                await load()
                let interval: Duration = isAdding ? .seconds(2) : .seconds(10)
                try? await Task.sleep(for: interval)
            }
        }
    }

    /// Additions to this list that are not yet entries: in progress, or failed.
    private var pending: [Addition] {
        additions.filter { addition in
            addition.list == list.slug && addition.status != .done && !entries.contains { $0.path == addition.path }
        }
    }

    private var isAdding: Bool {
        pending.contains { $0.status != .failed }
    }

    private var errorIsPresented: Binding<Bool> {
        Binding(
            get: { errorMessage != nil },
            set: { isPresented in
                if !isPresented {
                    errorMessage = nil
                }
            }
        )
    }

    private func load() async {
        if let detail = try? await CoreClient.list(list.slug) {
            entries = detail.entries
        }

        if let found = try? await RayClient.additions() {
            additions = found
        }
    }

    private func add(_ title: String) {
        let title = title.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !title.isEmpty else {
            return
        }

        Task {
            do {
                try await RayClient.startAddition(list: list.slug, title: title)
                await load()
            } catch {
                errorMessage = error.localizedDescription
            }
        }
    }
}
