import { For, createMemo } from "solid-js";

export default function HighlightedText(props: { text: string; query: string }) {
  const parts = createMemo(() => {
    const query = props.query.trim().toLowerCase(), text = props.text;
    if (!query) return [{ text, match: false }];
    const lower = text.toLowerCase(), result: { text: string; match: boolean }[] = [];
    let cursor = 0, index = lower.indexOf(query);
    while (index !== -1) {
      if (index > cursor) result.push({ text: text.slice(cursor, index), match: false });
      result.push({ text: text.slice(index, index + query.length), match: true });
      cursor = index + query.length;
      index = lower.indexOf(query, cursor);
    }
    if (cursor < text.length) result.push({ text: text.slice(cursor), match: false });
    return result;
  });
  return <For each={parts()}>{part => part.match ? <mark class="search-match">{part.text}</mark> : part.text}</For>;
}
