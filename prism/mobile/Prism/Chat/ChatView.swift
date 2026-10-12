import SwiftUI

struct ChatView: View {
    var body: some View {
        NavigationStack {
            ContentUnavailableView(
                "No Messages",
                systemImage: "bubble.left.and.bubble.right",
                description: Text("Messages that you send to Ray appear here.")
            )
            .navigationTitle("Chat")
        }
    }
}

#Preview {
    ChatView()
}
