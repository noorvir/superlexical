import SwiftUI

/// A page of the workspace that the Lists tab can show.
struct PageRoute: Hashable {
    let path: String
}

struct ListsView: View {
    @State private var lists: [ListSummary] = []
    @State private var isLoaded = false
    @State private var path = NavigationPath()
    @State private var isSettingsPresented = false

    var body: some View {
        NavigationStack(path: $path) {
            List(lists) { list in
                NavigationLink(value: list) {
                    VStack(alignment: .leading) {
                        Text(list.name)
                        Text(list.description)
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                }
                .badge(list.entryCount)
            }
            .overlay {
                if isLoaded && lists.isEmpty {
                    ContentUnavailableView(
                        "No Lists",
                        systemImage: "list.bullet",
                        description: Text("The workspace has no lists, or Core is not reachable.")
                    )
                }
            }
            .navigationTitle("Lists")
            .navigationDestination(for: ListSummary.self) { list in
                ListDetailView(list: list)
            }
            .navigationDestination(for: PageRoute.self) { route in
                PageView(path: route.path) { linked in
                    path.append(PageRoute(path: linked))
                }
            }
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button("Settings", systemImage: "gearshape") {
                        isSettingsPresented = true
                    }
                }
            }
            .sheet(isPresented: $isSettingsPresented) {
                SettingsView()
            }
            .refreshable {
                await load()
            }
            .task {
                await load()
            }
        }
    }

    private func load() async {
        lists = (try? await CoreClient.lists()) ?? []
        isLoaded = true
    }
}

#Preview {
    ListsView()
}
