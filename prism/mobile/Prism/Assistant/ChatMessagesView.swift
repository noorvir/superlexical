import SwiftUI

struct ChatMessagesView: View {
    let messages: [AssistantModel.Message]
    let isAnswering: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            ForEach(messages) { message in
                switch message.role {
                case .user:
                    Text(message.text)
                        .padding(.horizontal, 14)
                        .padding(.vertical, 10)
                        .background(.tint.opacity(0.15), in: .rect(cornerRadius: 18))
                        .frame(maxWidth: .infinity, alignment: .trailing)
                case .assistant:
                    MarkdownText(message.text)
                        .textSelection(.enabled)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }

            if isAnswering && messages.last?.role != .assistant {
                ProgressView()
            }
        }
    }
}
