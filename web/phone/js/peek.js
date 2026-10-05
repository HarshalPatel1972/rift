/* Screen Peek: see the PC screen on the phone, and tap right on it.
 *
 * One stream at a time, chosen by what's on screen:
 *   Peek tab  → whole screen / follow pointer / follow typing (user's pick)
 *   Type tab  → a strip that follows the text caret (if enabled)
 *   otherwise → off (saves battery and bandwidth)
 * Frames are flow-controlled: we ack each one after it's decoded and shown.
 */
import { $, $$, haptic, prefs, savePrefs } from './util.js';
import { FLAG, PEEK, link, sendAck, sendPeek, sendPoint } from './link.js';

const QUALITY_CAP = { saver: 720, balanced: 1280, crisp: 1920 };

const stage = $('#peek-stage');
const stageImg = $('#peek-img');
const ring = $('#peek-ring');
const empty = $('#peek-empty');
const mini = $('#type-peek');
const miniImg = $('img', mini);
const zoom = $('#peek-zoom');

let view = 'type';
let current = ''; // last request signature, to avoid resending
let target = null; // the <img> frames go to
let urls = new WeakMap();
let lastFrame = null; // { w, h, cx, cy }

const allowed = () => (link.flags & FLAG.PEEK) !== 0;

/** zoom slider 0..100 → screen span in px (more zoom = narrower window). */
const spanFor = (z) => Math.round(1800 - (z / 100) * 1440);

function desired() {
  if (!link.ready || !allowed() || document.visibilityState !== 'visible') return null;
  if (view === 'peek') return { mode: prefs.peekMode, el: stage, img: stageImg, span: spanFor(prefs.peekZoom) };
  if (view === 'type' && prefs.typePeek) return { mode: PEEK.CARET, el: mini, img: miniImg, span: 900 };
  return null;
}

/** Re-evaluates what to stream; cheap to call often. */
export function refreshPeek() {
  const showMini = view === 'type' && prefs.typePeek && link.ready && allowed();
  mini.hidden = !showMini;
  $('#peek-zoom-row').hidden = prefs.peekMode === PEEK.SCREEN;
  if (view === 'peek') renderEmpty();

  const d = desired();
  if (!d) {
    if (current !== 'off' && link.ready) sendPeek(PEEK.OFF, 0, 0, 0);
    current = 'off';
    target = null;
    return;
  }
  const r = d.el.getBoundingClientRect();
  if (!r.width || !r.height) return; // not laid out yet
  const dpr = Math.min(devicePixelRatio || 1, 3);
  const maxW = Math.min(Math.round(r.width * dpr), QUALITY_CAP[prefs.quality] || 1280);
  const aspect = Math.round((r.height / r.width) * 1000);
  const sig = `${d.mode}:${maxW}:${d.span}:${aspect}`;
  target = d.img;
  if (sig === current) return;
  current = sig;
  sendPeek(d.mode, maxW, d.span, aspect);
}

function renderEmpty() {
  const ok = link.ready && allowed();
  if (!ok) {
    empty.hidden = false;
    stageImg.hidden = true;
    ring.hidden = true;
    $('.peek-empty-emoji', empty).textContent = link.ready ? '🙈' : '🔌';
    $('b', empty).textContent = link.ready ? 'Peek is off on your PC' : 'Not connected';
    $('span', empty).textContent = link.ready
      ? 'Turn on "Let my phone see this screen" in RIFT on your PC.'
      : 'Waiting for your PC…';
  } else if (!stageImg.src) {
    empty.hidden = false;
    $('.peek-empty-emoji', empty).textContent = '👀';
    $('b', empty).textContent = 'Peeking…';
    $('span', empty).textContent = 'Your screen shows up here.';
  }
}

function onFrame(p) {
  const v = new DataView(p.buffer, p.byteOffset, p.byteLength);
  const id = v.getUint32(0, true);
  const w = v.getUint16(4, true), h = v.getUint16(6, true);
  const cx = v.getInt16(8, true), cy = v.getInt16(10, true);
  const img = target;
  if (!img) { sendAck(id); return; }

  const url = URL.createObjectURL(new Blob([p.subarray(12)], { type: 'image/jpeg' }));
  const prev = urls.get(img);
  const done = () => {
    if (prev) URL.revokeObjectURL(prev);
    sendAck(id);
  };
  const probe = new Image();
  probe.src = url;
  probe.decode().then(() => {
    img.src = url;
    urls.set(img, url);
    if (img === stageImg) {
      stageImg.hidden = false;
      empty.hidden = true;
      // Whole-screen frames: size the stage to the screen's shape so the mini
      // trackpad gets the leftover room instead of black bars.
      const fit = prefs.peekMode === PEEK.SCREEN;
      stage.classList.toggle('fit', fit);
      if (fit) stage.style.setProperty('--ar', `${w} / ${h}`);
      lastFrame = { w, h, cx, cy };
      placeRing();
    }
    done();
  }, () => { URL.revokeObjectURL(url); sendAck(id); });
}

/** The on-screen rectangle of the picture inside the object-fit: contain box. */
function contentRect() {
  const r = stageImg.getBoundingClientRect();
  const nw = stageImg.naturalWidth, nh = stageImg.naturalHeight;
  if (!nw || !nh) return r;
  const k = Math.min(r.width / nw, r.height / nh);
  const w = nw * k, h = nh * k;
  return { left: r.left + (r.width - w) / 2, top: r.top + (r.height - h) / 2, width: w, height: h };
}

function placeRing() {
  if (!lastFrame || stageImg.hidden) return;
  const { w, h, cx, cy } = lastFrame;
  const inside = cx >= 0 && cy >= 0 && cx < w && cy < h;
  ring.hidden = !inside;
  if (!inside) return;
  const s = stage.getBoundingClientRect();
  const r = contentRect();
  ring.style.left = `${r.left - s.left + (cx / w) * r.width}px`;
  ring.style.top = `${r.top - s.top + (cy / h) * r.height}px`;
}

function ripple(x, y) {
  const s = stage.getBoundingClientRect();
  const d = document.createElement('div');
  d.className = 'peek-ripple';
  d.style.left = `${x - s.left}px`;
  d.style.top = `${y - s.top}px`;
  stage.append(d);
  setTimeout(() => d.remove(), 520);
}

/* Tap = click there · long-press = right-click · drag = point (no click). */
function attachStageGestures() {
  let start = null;
  let pressTimer = 0;
  let pointing = false;
  let raf = 0;
  let pending = null;

  const norm = (e) => {
    const r = contentRect();
    return { x: (e.clientX - r.left) / r.width, y: (e.clientY - r.top) / r.height };
  };
  const onImage = (n) => n.x >= 0 && n.x <= 1 && n.y >= 0 && n.y <= 1;

  stage.addEventListener('pointerdown', (e) => {
    if (stageImg.hidden || !e.isPrimary) return;
    e.preventDefault();
    stage.setPointerCapture(e.pointerId);
    start = { x: e.clientX, y: e.clientY, t: e.timeStamp, fired: false };
    pointing = false;
    pressTimer = setTimeout(() => {
      const n = norm(e);
      if (!start || pointing || !onImage(n)) return;
      start.fired = true;
      sendPoint(n.x, n.y, 3);
      haptic(25);
      ripple(e.clientX, e.clientY);
    }, 480);
  });
  stage.addEventListener('pointermove', (e) => {
    if (!start || start.fired) return;
    if (!pointing && Math.hypot(e.clientX - start.x, e.clientY - start.y) > 10) {
      pointing = true;
      clearTimeout(pressTimer);
    }
    if (pointing) {
      pending = norm(e);
      if (!raf) raf = requestAnimationFrame(() => {
        raf = 0;
        if (pending && onImage(pending)) sendPoint(pending.x, pending.y, 0);
      });
    }
  });
  const end = (e) => {
    clearTimeout(pressTimer);
    if (!start) return;
    const s = start;
    start = null;
    if (s.fired || pointing || e.type !== 'pointerup') return;
    const n = norm(e);
    if (!onImage(n)) return;
    sendPoint(n.x, n.y, 1);
    haptic(8);
    ripple(e.clientX, e.clientY);
  };
  stage.addEventListener('pointerup', end);
  stage.addEventListener('pointercancel', end);
  stage.addEventListener('contextmenu', (e) => e.preventDefault());
}

export function setPeekView(v) {
  view = v;
  requestAnimationFrame(refreshPeek);
}

export function initPeek() {
  link.on('frame', onFrame);
  link.on('ready', () => { current = ''; refreshPeek(); });
  link.on('ping', () => refreshPeek());
  document.addEventListener('visibilitychange', () => { current = ''; refreshPeek(); });
  addEventListener('resize', () => { refreshPeek(); placeRing(); });

  $$('[data-peek]').forEach((b) => {
    b.addEventListener('click', () => {
      prefs.peekMode = Number(b.dataset.peek);
      stage.classList.remove('fit');
      savePrefs();
      haptic();
      syncSeg();
      refreshPeek();
    });
  });
  const syncSeg = () => $$('[data-peek]').forEach((b) =>
    b.setAttribute('aria-checked', String(Number(b.dataset.peek) === prefs.peekMode)));
  syncSeg();

  zoom.value = prefs.peekZoom;
  let zt = 0;
  zoom.addEventListener('input', () => {
    prefs.peekZoom = Number(zoom.value);
    clearTimeout(zt);
    zt = setTimeout(() => { savePrefs(); refreshPeek(); }, 120);
  });

  attachStageGestures();
}

/** Debug snapshot (window.rift.peek()). */
export const peekState = () => ({ view, current, target: target?.id || target?.alt || null, allowed: allowed(), ready: link.ready });
