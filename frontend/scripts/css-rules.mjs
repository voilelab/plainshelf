/** Returns the CSS of a file: a .vue file's <style> blocks, or the whole file. */
export function styleSource(source, file) {
  if (!file.endsWith('.vue')) {
    return source;
  }
  return [...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((match) => match[1]).join('\n');
}

/**
 * Returns every leaf rule as `{ selector, body }`, descending into at-rule
 * blocks rather than mis-reading them. A flat regular expression would take
 * `@media (…) {` as a selector and swallow the first rule inside it, which is
 * the kind of silent miss a lint cannot afford.
 */
export function rules(css) {
  const withoutComments = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const found = [];
  let head = '';

  for (let i = 0; i < withoutComments.length; i += 1) {
    const char = withoutComments[i];
    if (char === '{') {
      const body = blockAt(withoutComments, i);
      if (body.text.includes('{')) {
        // An at-rule block: its contents are rules of their own.
        found.push(...rules(body.text));
      } else {
        found.push({ selector: head.trim(), body: body.text });
      }
      i = body.end;
      head = '';
      continue;
    }
    if (char === '}') {
      head = '';
      continue;
    }
    head += char;
  }

  return found;
}

/** Returns the text between the brace at `open` and its match, plus that index. */
function blockAt(css, open) {
  let depth = 0;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === '{') depth += 1;
    if (css[i] === '}') {
      depth -= 1;
      if (depth === 0) {
        return { text: css.slice(open + 1, i), end: i };
      }
    }
  }
  return { text: css.slice(open + 1), end: css.length };
}
