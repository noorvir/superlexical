import SwiftUI

struct RootView: View {
    var body: some View {
        TabView {
            Tab("Lists", systemImage: "list.bullet") {
                ListsView()
            }

            Tab("Chat", systemImage: "bubble.left.and.bubble.right") {
                ChatView()
            }

            Tab(role: .search) {
                SearchView()
            }
        }
        .tabBarMinimizeBehavior(.onScrollDown)
    }
}

#Preview {
    RootView()
}
