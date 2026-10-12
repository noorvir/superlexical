import SwiftUI

struct ListsView: View {
    @State private var isSettingsPresented = false

    var body: some View {
        NavigationStack {
            ContentUnavailableView(
                "No Lists",
                systemImage: "list.bullet",
                description: Text("Lists that you create appear here.")
            )
            .navigationTitle("Lists")
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
        }
    }
}

#Preview {
    ListsView()
}
