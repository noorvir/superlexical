import SwiftUI

struct SearchResultView: View {
    let result: AssistantModel.SearchResult

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            if !result.images.isEmpty {
                ScrollView(.horizontal) {
                    LazyHStack(spacing: 8) {
                        ForEach(result.images) { image in
                            AsyncImage(url: image.thumbnailUrl) { loaded in
                                loaded
                                    .resizable()
                                    .scaledToFill()
                            } placeholder: {
                                Rectangle()
                                    .fill(.quaternary)
                            }
                            .frame(width: 150, height: 190)
                            .clipShape(.rect(cornerRadius: 16))
                            .accessibilityLabel(image.title)
                        }
                    }
                }
                .scrollIndicators(.hidden)
                .scrollClipDisabled()
            }

            Text(result.text.first?.title ?? result.query)
                .font(.title.bold())

            if result.summary.isEmpty {
                ProgressView()
            } else {
                Text(result.summary)
                    .font(.body)
                    .textSelection(.enabled)
            }

            ForEach(result.text, id: \.url) { source in
                Link(destination: source.url) {
                    Label(source.source, systemImage: "book.pages")
                }
                .buttonStyle(.bordered)
                .buttonBorderShape(.capsule)
            }
        }
    }
}
