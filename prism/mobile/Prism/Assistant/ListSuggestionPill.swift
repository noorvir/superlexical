import SwiftUI

/// A small glass pill that suggests a list for the current search.
///
/// A tap adds the topic to the suggested list. A long press shows the other
/// lists. A swipe in any direction dismisses the pill.
struct ListSuggestionPill: View {
    let suggestion: ListSuggestion
    let lists: [ListSummary]
    let add: (String) -> Void
    let dismiss: () -> Void

    @State private var offset = CGSize.zero

    var body: some View {
        Menu {
            Section("Add to Another List") {
                ForEach(otherLists) { list in
                    Button(list.name) {
                        add(list.slug)
                    }
                }
            }

            Button("Dismiss", systemImage: "xmark", action: dismiss)
        } label: {
            Text(suggestion.list.name)
                .font(.subheadline.weight(.medium))
                .padding(.horizontal, 6)
        } primaryAction: {
            add(suggestion.list.slug)
        }
        .buttonStyle(.glass)
        .buttonBorderShape(.capsule)
        .accessibilityLabel("Add \(suggestion.title) to \(suggestion.list.name)")
        .accessibilityAction(named: "Dismiss", dismiss)
        .offset(offset)
        .opacity(1 - min(distance / 160, 0.7))
        .simultaneousGesture(
            DragGesture(minimumDistance: 12)
                .onChanged { drag in
                    offset = drag.translation
                }
                .onEnded { drag in
                    let travelled = hypot(drag.translation.width, drag.translation.height)
                    if travelled > 60 {
                        dismiss()
                    } else {
                        withAnimation(.snappy) {
                            offset = .zero
                        }
                    }
                }
        )
    }

    private var otherLists: [ListSummary] {
        lists.filter { $0.slug != suggestion.list.slug }
    }

    private var distance: CGFloat {
        hypot(offset.width, offset.height)
    }
}
