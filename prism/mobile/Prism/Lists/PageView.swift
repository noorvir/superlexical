import SwiftUI

/// One Markdown page of the workspace: its fields and its text. A link to
/// another page of the workspace opens that page in the same navigation stack.
struct PageView: View {
    let path: String
    let open: (String) -> Void

    @State private var page: Page?
    @State private var isMissing = false

    var body: some View {
        List {
            if let page {
                if !page.fields.isEmpty {
                    Section {
                        ForEach(page.fields) { field in
                            LabeledContent(field.name.capitalized.replacing("_", with: " "), value: field.value)
                        }
                    }
                }

                Section {
                    MarkdownText(body(of: page))
                        .textSelection(.enabled)
                        .environment(\.openURL, OpenURLAction(handler: follow))
                }
            }
        }
        .overlay {
            if isMissing {
                ContentUnavailableView(
                    "Page Not Available",
                    systemImage: "doc.questionmark",
                    description: Text("Core does not have this page.")
                )
            }
        }
        .navigationTitle(page?.title ?? "")
        .navigationBarTitleDisplayMode(.inline)
        .task {
            do {
                page = try await CoreClient.page(path)
            } catch {
                isMissing = true
            }
        }
    }

    /// The text without its first heading, because the navigation bar shows the title.
    private func body(of page: Page) -> String {
        let lines = page.body.split(separator: "\n", omittingEmptySubsequences: false)
        guard let first = lines.firstIndex(where: { !$0.trimmingCharacters(in: .whitespaces).isEmpty }),
              lines[first].hasPrefix("# ")
        else {
            return page.body
        }

        return lines[(first + 1)...].joined(separator: "\n")
    }

    private func follow(_ url: URL) -> OpenURLAction.Result {
        guard url.scheme == nil else {
            return .systemAction
        }

        let folder = URL(filePath: "/" + path).deletingLastPathComponent()
        let target = folder.appending(path: url.path(percentEncoded: false)).standardizedFileURL
        open(String(target.path(percentEncoded: false).dropFirst()))
        return .handled
    }
}
