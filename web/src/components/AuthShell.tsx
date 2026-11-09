import { Show, onMount, type JSX } from "solid-js";
import Logo from "./Logo";
import { mediaQuery } from "../media";

export default function AuthShell(props: { children: JSX.Element }) {
  const wide = mediaQuery("(width >= 64rem)");
  const reducedMotion = mediaQuery("(prefers-reduced-motion: reduce)");
  let video!: HTMLVideoElement;
  onMount(() => {
    if (!video || reducedMotion()) return;
    video.muted = true;
    void video.play().catch(() => undefined);
  });
  return <div class="app-frame auth-frame">
    <div class="topbar"><a class="brand" href="/" aria-label="Shipper"><Logo /><span>Shipper</span></a></div>
    <main class="app-panel auth-panel">
      <Show when={wide()}>
        <aside class="auth-visual" aria-hidden="true">
          <video ref={video} class="auth-video" src="/auth-video.mp4" poster="/auth-video-poster.jpg" autoplay={!reducedMotion()} muted loop playsinline preload="auto" tabindex="-1" disablepictureinpicture />
          <span class="auth-visual-shade" />
        </aside>
      </Show>
      <section class="auth-stage">
        <div class="auth-stage-content">{props.children}</div>
      </section>
    </main>
  </div>;
}
