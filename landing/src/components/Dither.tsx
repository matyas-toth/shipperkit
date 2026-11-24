import { useEffect, useRef } from "react";

/*
  Real-time version of brand/shipdither.py: an 8x8 Bayer dither of brightness
  where each dot keeps the footage's own color. Sources in /media are
  pre-shrunk to 256x144 (one texel per dot or less), so the page never loads a
  full-size clip. The pointer lights up dots near it, and a click sends a ripple.
*/

export type Fade = "none" | "left" | "right" | "top" | "bottom" | "center" | "edges" | "bottom-left";

const FADES: Record<Fade, number> = { none: 0, left: 1, right: 2, top: 3, bottom: 4, center: 5, edges: 6, "bottom-left": 7 };

type Props = {
  src: string;
  still?: boolean;
  cell?: number;
  gap?: number;
  levels?: number;
  fade?: Fade;
  fadeFrom?: number;
  fadeTo?: number;
  /** Radius of a black clearing behind copy, in widths. 0 turns it off. */
  clear?: number;
  clearAt?: [number, number];
  exposure?: number;
  saturation?: number;
  tint?: number;
  focus?: [number, number];
  zoom?: number;
  round?: boolean;
  interactive?: boolean;
  bg?: string;
  className?: string;
  label?: string;
};

const VERT = `attribute vec2 p;void main(){gl_Position=vec4(p,0.,1.);}`;

const FRAG = `
precision highp float;
uniform sampler2D tex;
uniform vec2 res;
uniform vec2 texSize;
uniform float cell, gap, levels, exposure, saturation, tint, zoom, round_, fadeMode, fadeFrom, fadeTo, time, clearR;
uniform vec2 clearAt;
uniform vec2 focus;
uniform vec3 bg;
uniform vec3 mouse;
uniform vec4 ripple;

float bayer2(vec2 a){ a=floor(a); return fract(dot(a, vec2(.5, a.y*.75))); }
float bayer4(vec2 a){ return bayer2(.5*a)*.25 + bayer2(a); }
float bayer8(vec2 a){ return bayer4(.5*a)*.25 + bayer2(a); }

void main(){
  vec2 frag = vec2(gl_FragCoord.x, res.y - gl_FragCoord.y);
  vec2 grid = floor(frag / cell);
  vec2 local = frag - grid * cell;
  vec2 cells = floor(res / cell);
  vec2 center = (grid + .5) * cell;
  vec2 uv = center / res;

  // object-fit: cover around a focus point
  float ca = res.x / res.y, ta = texSize.x / texSize.y;
  vec2 scale = ca > ta ? vec2(1., ta / ca) : vec2(ca / ta, 1.);
  scale /= zoom;
  vec2 off = (1. - scale) * focus;
  vec3 c = texture2D(tex, off + uv * scale).rgb;

  // saturation
  float l0 = dot(c, vec3(.2126, .7152, .0722));
  c = mix(vec3(l0), c, saturation);
  c = pow(clamp(c, 0., 1.), vec3(1.15)) * exposure;

  // fades so headlines sit on clean black
  vec2 g = grid / max(cells, vec2(1.));
  float f = 1.;
  if (fadeMode == 1.) f = smoothstep(fadeFrom, fadeTo, g.x);
  else if (fadeMode == 2.) f = smoothstep(fadeFrom, fadeTo, 1. - g.x);
  else if (fadeMode == 3.) f = smoothstep(fadeFrom, fadeTo, g.y);
  else if (fadeMode == 4.) f = smoothstep(fadeFrom, fadeTo, 1. - g.y);
  else if (fadeMode == 5.) { vec2 d = (g - .5) * vec2(1.6, 2.2); f = smoothstep(fadeFrom, fadeTo, length(d)); }
  else if (fadeMode == 6.) { vec2 d = abs(g - .5) * 2.; f = 1. - smoothstep(fadeFrom, fadeTo, max(d.x * .9, d.y)); }
  else if (fadeMode == 7.) f = smoothstep(fadeFrom, fadeTo, max(g.x * 1.2, 1. - g.y));
  if (clearR > 0.) { vec2 d = (g - clearAt) * res / max(res.x, res.y) * vec2(1., res.x > res.y ? 1.9 : 1.); float r = length(d); f *= smoothstep(clearR * .55, clearR * 1.25, r); }
  c *= f;
  c *= 1. - .35 * pow(abs(g.y - .5) * 2., 3.);

  // pointer light and click ripple
  float lift = 0.;
  if (mouse.z > 0.) {
    float d = length(center - mouse.xy) / (cell * 9.);
    lift += mouse.z * .55 * exp(-d * d * 1.6);
  }
  if (ripple.z > 0.) {
    float r = length(center - ripple.xy);
    float front = ripple.w * cell * 1.1;
    float band = exp(-pow((r - front) / (cell * 2.2), 2.));
    lift += ripple.z * band * .7;
  }
  c = c * (1. + lift * 1.4) + vec3(lift * .18);
  c = clamp(c, 0., 1.);

  float Y = dot(c, vec3(.2126, .7152, .0722));
  float t = (bayer8(grid) + .5 / 64.);
  float L = levels - 1.;
  float k = clamp(floor(Y * L + t) / L, 0., 1.);
  vec3 chroma = c / max(Y, .18);
  chroma = chroma + (1. - min(Y / .18, 1.)) * (1. - chroma) * .5;
  vec3 q = clamp(chroma * k, 0., 1.);
  q = q * tint + vec3(k) * (1. - tint);
  vec3 dots = bg + q * (1. - bg);

  float inside;
  if (round_ > .5) {
    float rad = (cell - gap) * .5;
    inside = clamp(rad + .5 - length(local - vec2(rad)), 0., 1.);
  } else {
    inside = step(local.x, cell - gap) * step(local.y, cell - gap);
  }
  if (grid.x >= cells.x || grid.y >= cells.y) inside = 0.;
  gl_FragColor = vec4(mix(bg, dots, inside), 1.);
}`;

function hex(color: string): [number, number, number] {
  const n = parseInt(color.replace("#", ""), 16);
  return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255];
}

export default function Dither({
  src,
  still = false,
  cell = 12,
  gap = 2,
  levels = 3,
  fade = "none",
  fadeFrom = 0.05,
  fadeTo = 0.6,
  clear = 0,
  clearAt = [0.5, 0.45],
  exposure = 1,
  saturation = 1.15,
  tint = 1,
  focus = [0.5, 0.5],
  zoom = 1,
  round = false,
  interactive = true,
  bg = "#080808",
  className = "",
  label,
}: Props) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const poster = `/media/${src}.webp`;

  useEffect(() => {
    const el = canvas.current;
    if (!el) return;
    const gl = el.getContext("webgl", { antialias: false, premultipliedAlpha: false, alpha: false });
    if (!gl) return;
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    const shader = (type: number, source: string) => {
      const s = gl.createShader(type)!;
      gl.shaderSource(s, source);
      gl.compileShader(s);
      return s;
    };
    const program = gl.createProgram()!;
    gl.attachShader(program, shader(gl.VERTEX_SHADER, VERT));
    gl.attachShader(program, shader(gl.FRAGMENT_SHADER, FRAG));
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return;
    gl.useProgram(program);
    const buffer = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    const loc = gl.getAttribLocation(program, "p");
    gl.enableVertexAttribArray(loc);
    gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);

    const texture = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, texture);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);

    const u = (name: string) => gl.getUniformLocation(program, name);
    const U = {
      res: u("res"), texSize: u("texSize"), cell: u("cell"), gap: u("gap"), levels: u("levels"), exposure: u("exposure"),
      saturation: u("saturation"), tint: u("tint"), zoom: u("zoom"), round: u("round_"), fadeMode: u("fadeMode"),
      fadeFrom: u("fadeFrom"), fadeTo: u("fadeTo"), focus: u("focus"), bg: u("bg"), mouse: u("mouse"), ripple: u("ripple"), time: u("time"), clearR: u("clearR"), clearAt: u("clearAt"),
    };

    let dpr = 1;
    let cellPx = cell;
    let source: HTMLVideoElement | HTMLImageElement | null = null;
    let ready = false;
    let visible = false;
    let raf = 0;
    const pointer = { x: 0, y: 0, on: 0, target: 0 };
    const wave = { x: 0, y: 0, start: -1 };

    const resize = () => {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      const box = el.getBoundingClientRect();
      const compact = box.width < 640 ? 0.72 : 1;
      cellPx = Math.max(5, Math.round(cell * compact)) * dpr;
      el.width = Math.max(1, Math.round(box.width * dpr));
      el.height = Math.max(1, Math.round(box.height * dpr));
      gl.viewport(0, 0, el.width, el.height);
      draw();
    };

    const draw = () => {
      if (!ready || !source) return;
      gl.bindTexture(gl.TEXTURE_2D, texture);
      try {
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGB, gl.RGB, gl.UNSIGNED_BYTE, source);
      } catch {
        return;
      }
      const w = source instanceof HTMLVideoElement ? source.videoWidth : source.naturalWidth;
      const h = source instanceof HTMLVideoElement ? source.videoHeight : source.naturalHeight;
      gl.uniform2f(U.res, el.width, el.height);
      gl.uniform2f(U.texSize, w || 256, h || 144);
      gl.uniform1f(U.cell, cellPx);
      gl.uniform1f(U.gap, Math.max(1, Math.round(gap * dpr * (cellPx / (cell * dpr)))));
      gl.uniform1f(U.levels, levels);
      gl.uniform1f(U.exposure, exposure);
      gl.uniform1f(U.saturation, saturation);
      gl.uniform1f(U.tint, tint);
      gl.uniform1f(U.zoom, zoom);
      gl.uniform1f(U.round, round ? 1 : 0);
      gl.uniform1f(U.fadeMode, FADES[fade]);
      gl.uniform1f(U.fadeFrom, fadeFrom);
      gl.uniform1f(U.fadeTo, fadeTo);
      gl.uniform2f(U.focus, focus[0], focus[1]);
      gl.uniform1f(U.clearR, clear);
      gl.uniform2f(U.clearAt, clearAt[0], clearAt[1]);
      gl.uniform3fv(U.bg, hex(bg));
      gl.uniform3f(U.mouse, pointer.x * dpr, pointer.y * dpr, pointer.on);
      const age = wave.start < 0 ? -1 : (performance.now() - wave.start) / 1000;
      const strength = age < 0 || age > 1.6 ? 0 : 1 - age / 1.6;
      gl.uniform4f(U.ripple, wave.x * dpr, wave.y * dpr, strength, age * 34);
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      el.dataset.ready = "true";
    };

    const animating = () => pointer.on > 0.001 || pointer.target > 0 || (wave.start >= 0 && performance.now() - wave.start < 1600);

    const loop = () => {
      raf = 0;
      if (!visible) return;
      pointer.on += (pointer.target - pointer.on) * 0.14;
      draw();
      const playing = source instanceof HTMLVideoElement && !source.paused;
      if (playing || animating()) raf = requestAnimationFrame(loop);
    };
    const kick = () => {
      if (!raf && visible) raf = requestAnimationFrame(loop);
    };

    if (still || reduced) {
      const img = new Image();
      img.src = poster;
      img.onload = () => {
        source = img;
        ready = true;
        resize();
      };
    } else {
      const video = document.createElement("video");
      video.src = `/media/${src}.mp4`;
      video.muted = true;
      video.loop = true;
      video.playsInline = true;
      video.preload = "auto";
      video.setAttribute("muted", "");
      video.setAttribute("playsinline", "");
      video.addEventListener("loadeddata", () => {
        source = video;
        ready = true;
        resize();
        if (visible) video.play().then(kick).catch(() => {});
      });
      video.addEventListener("play", kick);
      source = null;
      (el as unknown as { _video?: HTMLVideoElement })._video = video;
    }

    const io = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      const video = (el as unknown as { _video?: HTMLVideoElement })._video;
      if (video) {
        if (visible && ready) video.play().then(kick).catch(() => {});
        else video.pause();
      }
      kick();
    });
    io.observe(el);
    const ro = new ResizeObserver(resize);
    ro.observe(el);

    // Listen on the window so text and buttons layered above still move the light.
    const inside = (event: PointerEvent) => {
      const box = el.getBoundingClientRect();
      const x = event.clientX - box.left;
      const y = event.clientY - box.top;
      return { x, y, ok: x >= 0 && y >= 0 && x <= box.width && y <= box.height };
    };
    const move = (event: PointerEvent) => {
      if (!interactive || !visible) return;
      const p = inside(event);
      if (!p.ok) {
        if (pointer.target) leave();
        return;
      }
      pointer.x = p.x;
      pointer.y = p.y;
      pointer.target = event.pointerType === "mouse" ? 1 : 0;
      kick();
    };
    const leave = () => {
      pointer.target = 0;
      kick();
    };
    const down = (event: PointerEvent) => {
      if (!interactive || !visible) return;
      const p = inside(event);
      if (!p.ok) return;
      wave.x = p.x;
      wave.y = p.y;
      wave.start = performance.now();
      kick();
    };
    window.addEventListener("pointermove", move, { passive: true });
    document.documentElement.addEventListener("pointerleave", leave);
    window.addEventListener("pointerdown", down, { passive: true });

    return () => {
      cancelAnimationFrame(raf);
      io.disconnect();
      ro.disconnect();
      window.removeEventListener("pointermove", move);
      document.documentElement.removeEventListener("pointerleave", leave);
      window.removeEventListener("pointerdown", down);
      const video = (el as unknown as { _video?: HTMLVideoElement })._video;
      video?.pause();
      video?.removeAttribute("src");
    };
  }, [src, still, cell, gap, levels, fade, fadeFrom, fadeTo, exposure, saturation, tint, zoom, round, interactive, bg, focus[0], focus[1], clear, clearAt[0], clearAt[1]]);

  return (
    <div className={`absolute inset-0 overflow-hidden ${className}`} style={{ background: bg }} role={label ? "img" : undefined} aria-label={label} aria-hidden={label ? undefined : true}>
      <canvas ref={canvas} className="peer absolute inset-0 size-full opacity-0 transition-opacity duration-700 data-[ready=true]:opacity-100" />
    </div>
  );
}
