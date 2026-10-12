import SwiftUI

/// The search and chat screen. The system search field at the bottom is the
/// input for the two modes.
struct AssistantView: View {
    @State private var model = AssistantModel()
    @State private var position = ScrollPosition(edge: .top)

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    if let result = model.result {
                        SearchResultView(result: result)
                    }

                    if model.mode == .chat {
                        ChatMessagesView(messages: model.messages, isAnswering: model.isAnswering)
                    }
                }
                .padding()
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .scrollPosition($position)
            .onChange(of: model.messages.last?.text) {
                withAnimation(.snappy) {
                    position.scrollTo(edge: .bottom)
                }
            }
            .overlay {
                if model.result == nil && model.messages.isEmpty {
                    emptyState
                }
            }
            .safeAreaBar(edge: .bottom) {
                if model.mode == .search, let suggestion = model.suggestion {
                    ListSuggestionPill(
                        suggestion: suggestion,
                        lists: model.lists,
                        add: model.add(to:),
                        dismiss: model.dismissSuggestion
                    )
                    .padding(.bottom, 8)
                    .transition(.blurReplace.combined(with: .move(edge: .bottom)))
                }
            }
            .animation(.snappy, value: model.suggestion?.title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    Picker("Mode", selection: $model.mode) {
                        ForEach(AssistantModel.Mode.allCases) { mode in
                            Text(mode.rawValue).tag(mode)
                        }
                    }
                    .pickerStyle(.segmented)
                    .frame(maxWidth: 220)
                }

                if model.mode == .search, model.result != nil, !model.lists.isEmpty {
                    ToolbarItem(placement: .topBarTrailing) {
                        Menu("Add to List", systemImage: "text.badge.plus") {
                            ForEach(model.lists) { list in
                                Button(list.name) {
                                    model.add(to: list.slug)
                                }
                            }
                        }
                    }
                }
            }
        }
        .searchable(text: $model.input, prompt: model.mode == .search ? "Search" : "Message Ray")
        .searchPresentationToolbarBehavior(.avoidHidingContent)
        .onSubmit(of: .search, model.submit)
        .sensoryFeedback(.success, trigger: model.additionCount)
        .task {
            await model.loadLists()
        }
        .alert("Cannot Complete the Request", isPresented: errorIsPresented) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(model.errorMessage ?? "")
        }
    }

    @ViewBuilder
    private var emptyState: some View {
        switch model.mode {
        case .search:
            ContentUnavailableView(
                "Search",
                systemImage: "magnifyingglass",
                description: Text("Find a person, place, or thing. Then add it to a list.")
            )
        case .chat:
            ContentUnavailableView(
                "Chat",
                systemImage: "bubble.left.and.bubble.right",
                description: Text("Send a message to Ray.")
            )
        }
    }

    private var errorIsPresented: Binding<Bool> {
        Binding(
            get: { model.errorMessage != nil },
            set: { isPresented in
                if !isPresented {
                    model.errorMessage = nil
                }
            }
        )
    }
}

#Preview {
    AssistantView()
}
