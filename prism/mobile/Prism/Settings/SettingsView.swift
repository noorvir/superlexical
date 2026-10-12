import SwiftUI

struct SettingsView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var monitor = ServiceMonitor()

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    ServiceRow(name: "Core", url: ServiceEndpoints.core, state: monitor.core)
                    ServiceRow(name: "Ray", url: ServiceEndpoints.ray, state: monitor.ray)
                } header: {
                    Text("Services")
                } footer: {
                    Text("Pull down to check the services again.")
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(role: .close) {
                        dismiss()
                    }
                }
            }
            .refreshable {
                await monitor.refresh()
            }
            .task {
                await monitor.refresh()
            }
        }
    }
}

private struct ServiceRow: View {
    let name: String
    let url: URL
    let state: ServiceState

    var body: some View {
        LabeledContent {
            switch state {
            case .checking:
                ProgressView()
            case .running:
                Label("Running", systemImage: "checkmark.circle.fill")
                    .foregroundStyle(.green)
            case .unreachable:
                Label("Not Reachable", systemImage: "exclamationmark.circle.fill")
                    .foregroundStyle(.red)
            }
        } label: {
            Text(name)
            Text(url.absoluteString)
        }
    }
}

#Preview {
    SettingsView()
}
