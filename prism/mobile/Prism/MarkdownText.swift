import SwiftUI

/// Shows Markdown text with headings, list items, and paragraphs. Inline
/// markup and links use the system Markdown support of `Text`.
struct MarkdownText: View {
    private let blocks: [Block]

    init(_ source: String) {
        blocks = Block.parse(source)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            ForEach(blocks) { block in
                switch block.kind {
                case .heading(let level):
                    Text(inline(block.text))
                        .font(level == 1 ? .title2.bold() : .headline)
                        .padding(.top, 4)
                case .item:
                    HStack(alignment: .firstTextBaseline, spacing: 8) {
                        Text("•")
                        Text(inline(block.text))
                    }
                case .paragraph:
                    Text(inline(block.text))
                }
            }
        }
    }

    private func inline(_ text: String) -> AttributedString {
        let options = AttributedString.MarkdownParsingOptions(interpretedSyntax: .inlineOnlyPreservingWhitespace)
        return (try? AttributedString(markdown: text, options: options)) ?? AttributedString(text)
    }

    private struct Block: Identifiable {
        enum Kind {
            case heading(Int)
            case item
            case paragraph
        }

        let id: Int
        let kind: Kind
        let text: String

        static func parse(_ source: String) -> [Block] {
            var blocks: [Block] = []
            var paragraph: [String] = []

            func endParagraph() {
                if !paragraph.isEmpty {
                    blocks.append(Block(id: blocks.count, kind: .paragraph, text: paragraph.joined(separator: " ")))
                    paragraph = []
                }
            }

            for line in source.split(separator: "\n", omittingEmptySubsequences: false) {
                let trimmed = line.trimmingCharacters(in: .whitespaces)

                if trimmed.isEmpty {
                    endParagraph()
                } else if trimmed.hasPrefix("#") {
                    endParagraph()
                    let level = trimmed.prefix { $0 == "#" }.count
                    let text = trimmed.dropFirst(level).trimmingCharacters(in: .whitespaces)
                    blocks.append(Block(id: blocks.count, kind: .heading(level), text: text))
                } else if trimmed.hasPrefix("- ") || trimmed.hasPrefix("* ") {
                    endParagraph()
                    blocks.append(Block(id: blocks.count, kind: .item, text: String(trimmed.dropFirst(2))))
                } else {
                    paragraph.append(trimmed)
                }
            }

            endParagraph()
            return blocks
        }
    }
}
