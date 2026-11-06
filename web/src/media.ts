import { createSignal, onCleanup, onMount } from "solid-js";

export function mediaQuery(query: string) {
  const media = window.matchMedia(query);
  const [matches, setMatches] = createSignal(media.matches);
  const update = () => setMatches(media.matches);
  onMount(() => { media.addEventListener("change", update); update(); });
  onCleanup(() => media.removeEventListener("change", update));
  return matches;
}
