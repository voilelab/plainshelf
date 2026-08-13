import MarkdownIt from 'markdown-it';
import {
  assetImageFromMarkdownLine,
  updateMarkdownFenceState,
  type MarkdownFenceState
} from './parseMarkdownBlocks';

export type ReaderMarkdownHtmlBlock = {
  type: 'html';
  /** Unsanitized renderer output. ReaderSafeHtml is the only permitted sink. */
  html: string;
};

export type ReaderMarkdownImageBlock = {
  type: 'image';
  name: string;
  alt: string;
};

export type ReaderMarkdownBlock = ReaderMarkdownHtmlBlock | ReaderMarkdownImageBlock;

const markdown = new MarkdownIt({
  html: true,
  breaks: true,
  linkify: false,
  typographer: false
});

// Links and arbitrary images were not part of the old reader and could make a
// book navigate or fetch from the network. A valid, standalone source asset is
// extracted before this renderer runs; every other spelling stays literal.
markdown.disable(['image', 'link', 'autolink']);

type RenderTokens = Parameters<typeof markdown.renderer.renderToken>[0];

function renderToken(tokens: RenderTokens, index: number): string {
  return markdown.renderer.renderToken(tokens, index, markdown.options);
}

markdown.renderer.rules.heading_open = (tokens, index) => {
  const sourceLevel = Number(tokens[index].tag.slice(1));
  tokens[index].tag = sourceLevel === 1 ? 'h2' : sourceLevel === 2 ? 'h3' : 'h4';
  tokens[index].attrSet(
    'class',
    sourceLevel === 1 ? 'reader-md-h1' : sourceLevel === 2 ? 'reader-md-h2' : 'reader-md-h3'
  );
  return renderToken(tokens, index);
};

markdown.renderer.rules.heading_close = (tokens, index) => {
  const sourceLevel = Number(tokens[index].tag.slice(1));
  tokens[index].tag = sourceLevel === 1 ? 'h2' : sourceLevel === 2 ? 'h3' : 'h4';
  return renderToken(tokens, index);
};

markdown.renderer.rules.paragraph_open = (tokens, index) => {
  tokens[index].attrJoin('class', 'reader-text-block');
  return renderToken(tokens, index);
};

markdown.renderer.rules.blockquote_open = (tokens, index) => {
  tokens[index].attrJoin('class', 'reader-text-block reader-text-quote');
  return renderToken(tokens, index);
};

for (const rule of ['bullet_list_open', 'ordered_list_open'] as const) {
  markdown.renderer.rules[rule] = (tokens, index) => {
    tokens[index].attrJoin('class', 'reader-md-list');
    return renderToken(tokens, index);
  };
}

markdown.renderer.rules.fence = (tokens, index) => {
  const content = markdown.utils.escapeHtml(tokens[index].content);
  return `<pre class="reader-md-code"><code>${content}</code></pre>\n`;
};

markdown.renderer.rules.code_block = markdown.renderer.rules.fence;

markdown.renderer.rules.code_inline = (tokens, index) =>
  `<code class="reader-md-inline-code">${markdown.utils.escapeHtml(tokens[index].content)}</code>`;

markdown.renderer.rules.hr = () => '<hr class="reader-md-hr">\n';

// plot is a transparent authoring wrapper, not an element in the rendered
// document. Removing standalone markers before Markdown parsing lets the prose
// inside become ordinary paragraphs instead of relying on white-space CSS to
// preserve the raw HTML block's text nodes.
const PLOT_MARKER_LINE_RE = /^\s*<\/?plot(?:\s[^>]*)?>\s*$/i;

function renderHtmlBlock(source: string): ReaderMarkdownHtmlBlock | null {
  if (!source.trim()) return null;
  const html = markdown.render(source);
  return html.trim() ? { type: 'html', html } : null;
}

/**
 * Produces reader blocks while keeping local illustrations as Vue components.
 * All other Markdown, including raw HTML, remains an untrusted HTML string and
 * must pass through ReaderSafeHtml before it reaches the DOM.
 */
export function renderMarkdownBlocks(source: string): ReaderMarkdownBlock[] {
  if (!source.trim()) return [];

  const blocks: ReaderMarkdownBlock[] = [];
  const textBuffer: string[] = [];
  let fence: MarkdownFenceState | null = null;

  const flushText = (): void => {
    const block = renderHtmlBlock(textBuffer.join('\n'));
    if (block) blocks.push(block);
    textBuffer.length = 0;
  };

  for (const line of source.split(/\r?\n/)) {
    const transition = updateMarkdownFenceState(line, fence);

    if (!fence && !transition.boundary) {
      if (PLOT_MARKER_LINE_RE.test(line)) {
        textBuffer.push('');
        continue;
      }

      const image = assetImageFromMarkdownLine(line);
      if (image) {
        flushText();
        blocks.push({ type: 'image', ...image });
        continue;
      }
    }

    textBuffer.push(line);
    if (transition.boundary) fence = transition.state;
  }

  flushText();
  return blocks;
}
