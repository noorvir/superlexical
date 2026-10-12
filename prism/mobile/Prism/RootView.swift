import SwiftUI

struct RootView: View {
    var body: some View {
        TabView {
            Tab("Lists", systemImage: "list.bullet") {
                ListsView()
            }

            Tab(role: .search) {
                AssistantView()
            }
        }
    }
}

#Preview {
    RootView()
}
