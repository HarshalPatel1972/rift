/* Sparks: every input you send flies from your thumb up into the rift.
 * It's feedback, not decoration: you see that the tap left your phone.
 */
import { $ } from './util.js';
import { OP, link } from './link.js';
import { flare } from './seam.js';

const canvas = $('#sparks');
const ctx = canvas.getContext('2d');
const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
const COLORS = ['#ffd25e', '#ff9a4d', '#ff7a3d', '#ff5a6a', '#ff3d7f'];

let parts = [];
let raf = 0;
let dpr = 1;
let origin = { x: innerWidth / 2, y: innerHeight * 0.75 };
let last = 0;

function resize() {
  dpr = Math.min(devicePixelRatio || 1, 2);
  canvas.width = innerWidth * dpr;
  canvas.height = innerHeight * dpr;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function seamY() {
  const r = $('#seam').getBoundingClientRect();
  return r.top + r.height / 2;
}

/** Launches n sparks from (x, y) toward the seam. */
export function burst(n, x = origin.x, y = origin.y) {
  if (reduce || document.hidden) return;
  const ty = seamY();
  if (y <= ty + 4) return; // nothing to cross
  for (let i = 0; i < n; i++) {
    parts.push({
      x0: x + (Math.random() - 0.5) * 16,
      y0: y,
      x1: x + (Math.random() - 0.5) * 70,
      y1: ty + (Math.random() - 0.5) * 4,
      t: -Math.random() * 0.25, // staggered start
      speed: 1 / (300 + Math.random() * 260),
      r: 1.2 + Math.random() * 1.8,
      c: COLORS[(Math.random() * COLORS.length) | 0],
    });
  }
  if (parts.length > 160) parts = parts.slice(-160);
  if (!raf) raf = requestAnimationFrame(tick);
}

let prev = 0;
function tick(now) {
  const dt = prev ? Math.min(now - prev, 50) : 16;
  prev = now;
  ctx.clearRect(0, 0, innerWidth, innerHeight);
  let landed = false;
  for (const p of parts) {
    p.t += dt * p.speed;
    if (p.t <= 0) continue;
    const t = Math.min(p.t, 1);
    const e = t * t * (3 - 2 * t); // ease in-out
    const x = p.x0 + (p.x1 - p.x0) * e + Math.sin(t * 9 + p.r) * 4 * (1 - t);
    const y = p.y0 + (p.y1 - p.y0) * e;
    ctx.globalAlpha = 1 - t * 0.6;
    ctx.fillStyle = p.c;
    ctx.shadowBlur = 8;
    ctx.shadowColor = p.c;
    ctx.beginPath();
    ctx.arc(x, y, p.r * (1 - t * 0.5), 0, Math.PI * 2);
    ctx.fill();
    if (p.t >= 1 && !p.done) { p.done = true; landed = true; }
  }
  ctx.globalAlpha = 1;
  ctx.shadowBlur = 0;
  parts = parts.filter((p) => p.t < 1);
  if (landed) flare();
  if (parts.length) raf = requestAnimationFrame(tick);
  else { raf = 0; prev = 0; ctx.clearRect(0, 0, innerWidth, innerHeight); }
}

// How much light each kind of input throws.
const WEIGHT = {
  [OP.EDIT]: 4, [OP.KEY]: 6, [OP.BUTTON]: 7, [OP.ACTION]: 14,
  [OP.MOVE]: 1, [OP.SCROLL]: 1,
};

export function initSparks() {
  resize();
  addEventListener('resize', resize);
  // Sparks start wherever your thumb last was.
  addEventListener('pointerdown', (e) => { origin = { x: e.clientX, y: e.clientY }; }, { capture: true, passive: true });
  addEventListener('pointermove', (e) => { if (e.buttons || e.pointerType === 'touch') origin = { x: e.clientX, y: e.clientY }; }, { capture: true, passive: true });
  $('#editor').addEventListener('input', () => {
    const r = $('#editor').getBoundingClientRect();
    origin = { x: r.left + r.width * (0.3 + Math.random() * 0.4), y: r.top + 30 };
  });

  link.on('sent', (op) => {
    const n = WEIGHT[op];
    if (!n) return;
    const now = performance.now();
    // Continuous gestures (move, scroll) get a trickle, not a firehose.
    if (n === 1 && now - last < 90) return;
    last = now;
    burst(n);
  });
}
