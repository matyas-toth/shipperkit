// Shared open motion for menus: a quick rise to most of the height, then a short
// ease-out settle. The caller owns sizing and state.
export function settleMenu(node: HTMLElement, maxHeight: number, done?: () => void) {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    done?.();
    return;
  }
  const full = Math.min(node.scrollHeight, maxHeight);
  node.style.overflow = "hidden";
  node.style.height = "0px";
  const snap = Math.round(full * 0.75);
  const animation = node.animate(
    [
      { height: "0px", opacity: 0.55 },
      { height: `${snap}px`, opacity: 1, offset: 0.45 },
      { height: `${full}px`, opacity: 1 },
    ],
    { duration: 220, easing: "cubic-bezier(0.22, 1, 0.36, 1)" },
  );
  animation.onfinish = () => {
    node.style.height = "";
    node.style.overflow = "";
    done?.();
  };
}
