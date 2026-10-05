/* THERE: the portal, a live window into the computer above the seam.
 *
 * Every mode keeps one open, so you always see what your hands affect. Each
 * mode remembers its lens: Type follows the text caret, Touch follows the
 * pointer, Media and Keys show the whole screen. Tap the picture to click
 * there, hold to right-click, drag to point.
 *
 * Frames are flow-controlled: each is acked once decoded and shown. The
 * stream stops whenever the portal is folded or the phone is put away.
 */
import { $, $$, haptic, pc, prefs, savePrefs } from './util.js';
import { FLAG, PEEK, link, sendAck, sendPeek, sendPoint } from './link.js';
import { portalHeight } from './seam.js';

const QUALITY_CAP = { saver: 720, balanced: 1280, crisp: 1920 };
const DEFAULT_LENS = { type: PEEK.CARET, touch: PEEK.CURSOR, media: PEEK.SCREEN, keys: PEEK.SCREEN };

const portal = $('#portal');
const img = $('#portal-img');
const ring = $('#portal-ring');
const empty = $('#portal-empty');

let mode = 'type';
let current = '';
let lastFrame = null;
let url = '';

const allowed = () => (link.flags & FLAG.PEEK) !== 0;
const lens = () => prefs.lens?.[mode] || DEFAULT_LENS[mode];
/** zoom 0..100 → screen span in px (more zoom = narrower window). */
const span = () => Math.round(1800 - (prefs.peekZoom / 100) * 1500);

/** Re-evaluates what to stream; cheap to call often. */
export function refreshPortal() {
  portal.dataset.lens = lens();
  $$('[data-lens]').forEach((b) => b.setAttribute('aria-checked', String(Number(b.dataset.lens) === lens())));
  renderEmpty();

  const h = portalHeight();
  const on = link.ready && allowed() && document.visibilityState === 'visible' && h > 0;
  if (!on) {
    if (current !== 'off' && link.ready) sendPeek(PEEK.OFF, 0, 0, 0);
    current = 'off';
    return;
  }
  const r = portal.getBoundingClientRect();
  const dpr = Math.min(devicePixelRatio || 1, 3);
  const maxW = Math.min(Math.round(r.width * dpr), QUALITY_CAP[prefs.quality] || 1280);
  const aspect = Math.round((r.height / r.width) * 1000);
  const sig = `${lens()}:${maxW}:${span()}:${aspect}`;
  if (sig === current) return;
  current = sig;
  sendPeek(lens(), maxW, span(), aspect);
}

function renderEmpty() {
  const title = $('#portal-empty-title');
  const text = $('#portal-empty-text');
  if (!link.ready) {
    title.textContent = 'The other side';
    text.textContent = pc('Your PC shows up here once the rift is open.');
  } else if (!allowed()) {
    title.textContent = 'This window is shut';
    text.textContent = pc('Turn on “Let my phone see this screen” in RIFT on your PC.');
  } else {
    title.textContent = 'Looking through…';
    text.textContent = '';
  }
  const showing = link.ready && allowed() && !!img.src;
  empty.hidden = showing;
  img.hidden = !showing;
  if (!showing) ring.hidden = true;
}

function onFrame(p) {
  const v = new DataView(p.buffer, p.byteOffset, p.byteLength);
  const id = v.getUint32(0, true);
  const w = v.getUint16(4, true), h = v.getUint16(6, true);
  const cx = v.getInt16(8, true), cy = v.getInt16(10, true);
  if (current === 'off') { sendAck(id); return; }

  const next = URL.createObjectURL(new Blob([p.subarray(12)], { type: 'image/jpeg' }));
  const probe = new Image();
  probe.src = next;
  probe.decode().then(() => {
    img.src = next;
    if (url) URL.revokeObjectURL(url);
    url = next;
    lastFrame = { w, h, cx, cy };
    renderEmpty();
    placeRing();
    sendAck(id);
  }, () => { URL.revokeObjectURL(next); sendAck(id); });
}

/** The on-screen rectangle of the picture inside the object-fit: contain box. */
function contentRect() {
  const r = img.getBoundingClientRect();
  const nw = img.naturalWidth, nh = img.naturalHeight;
  if (!nw || !nh) return r;
  const k = Math.min(r.width / nw, r.height / nh);
  const w = nw * k, h = nh * k;
  return { left: r.left + (r.width - w) / 2, top: r.top + (r.height - h) / 2, width: w, height: h };
}

function placeRing() {
  if (!lastFrame || img.hidden) return;
  const { w, h, cx, cy } = lastFrame;
  const inside = cx >= 0 && cy >= 0 && cx < w && cy < h;
  ring.hidden = !inside;
  if (!inside) return;
  const s = portal.getBoundingClientRect();
  const r = contentRect();
  ring.style.left = `${r.left - s.left + (cx / w) * r.width}px`;
  ring.style.top = `${r.top - s.top + (cy / h) * r.height}px`;
}

function ripple(x, y) {
  const s = portal.getBoundingClientRect();
  const d = document.createElement('div');
  d.className = 'portal-ripple';
  d.style.left = `${x - s.left}px`;
  d.style.top = `${y - s.top}px`;
  portal.append(d);
  setTimeout(() => d.remove(), 560);
}

/* Tap = click there · hold = right-click · drag = point (no click). */
function attachGestures() {
  let start = null;
  let timer = 0;
  let pointing = false;
  let raf = 0;
  let pending = null;

  const norm = (e) => {
    const r = contentRect();
    return { x: (e.clientX - r.left) / r.width, y: (e.clientY - r.top) / r.height };
  };
  const onPicture = (n) => n.x >= 0 && n.x <= 1 && n.y >= 0 && n.y <= 1;

  portal.addEventListener('pointerdown', (e) => {
    if (img.hidden || !e.isPrimary || e.target.closest('.lens')) return;
    e.preventDefault();
    portal.setPointerCapture(e.pointerId);
    start = { x: e.clientX, y: e.clientY, fired: false };
    pointing = false;
    timer = setTimeout(() => {
      const n = norm(e);
      if (!start || pointing || !onPicture(n)) return;
      start.fired = true;
      sendPoint(n.x, n.y, 3);
      haptic(25);
      ripple(e.clientX, e.clientY);
    }, 480);
  });
  portal.addEventListener('pointermove', (e) => {
    if (!start || start.fired) return;
    if (!pointing && Math.hypot(e.clientX - start.x, e.clientY - start.y) > 10) {
      pointing = true;
      clearTimeout(timer);
    }
    if (pointing) {
      pending = norm(e);
      if (!raf) raf = requestAnimationFrame(() => {
        raf = 0;
        if (pending && onPicture(pending)) sendPoint(pending.x, pending.y, 0);
      });
    }
  });
  const end = (e) => {
    clearTimeout(timer);
    if (!start) return;
    const s = start;
    start = null;
    if (s.fired || pointing || e.type !== 'pointerup') return;
    const n = norm(e);
    if (!onPicture(n)) return;
    sendPoint(n.x, n.y, 1);
    haptic(8);
    ripple(e.clientX, e.clientY);
  };
  portal.addEventListener('pointerup', end);
  portal.addEventListener('pointercancel', end);
  portal.addEventListener('contextmenu', (e) => e.preventDefault());
}

export function setPortalMode(m) {
  mode = m;
  refreshPortal();
}

export function initPortal() {
  link.on('frame', onFrame);
  link.on('ready', () => { current = ''; refreshPortal(); });
  link.on('ping', () => refreshPortal());
  link.on('status', () => refreshPortal());
  document.addEventListener('visibilitychange', () => { current = ''; refreshPortal(); });
  addEventListener('resize', placeRing);

  $$('[data-lens]').forEach((b) => b.addEventListener('click', () => {
    prefs.lens = { ...(prefs.lens || {}), [mode]: Number(b.dataset.lens) };
    savePrefs();
    haptic(6);
    refreshPortal();
  }));
  $$('[data-zoom]').forEach((b) => b.addEventListener('click', () => {
    prefs.peekZoom = Math.min(100, Math.max(0, prefs.peekZoom + Number(b.dataset.zoom) * 15));
    savePrefs();
    haptic(6);
    refreshPortal();
  }));
  attachGestures();
  refreshPortal();
}

/** Debug snapshot (window.rift.portal()). */
export const portalState = () => ({ mode, lens: lens(), current, allowed: allowed(), ready: link.ready, height: portalHeight() });
