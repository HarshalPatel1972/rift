/* The seam: the rift between THERE (the portal) and HERE (your controls).
 *
 * - It's drawn as a glowing tear whose light is the connection state
 *   (body[data-link] drives the CSS).
 * - Drag it to resize the portal; tap it to fold the portal away or back.
 * - It carries one short status label (paused, sleep timer, reconnecting).
 */
import { $, haptic, prefs, savePrefs } from './util.js';
import { tear } from './tear.js';

const seam = $('#seam');
const portal = $('#portal');
const app = $('.app');

const DEFAULTS = { type: 0.26, touch: 0.3, media: 0.3, keys: 0.2 };
const MIN_OPEN = 56;   // px; below this the portal folds shut
const MIN_HERE = 190;  // px always left for the controls

let mode = 'type';
let onChange = () => {};
const labels = new Map(); // key → text, highest priority first by insertion

/** Space the portal may take: the app minus header, seam, modes and controls. */
function room() {
  const fixed = $('.top').offsetHeight + seam.offsetHeight + $('.modes').offsetHeight;
  return Math.max(0, app.clientHeight - fixed - MIN_HERE);
}

function fraction() {
  const f = prefs.portal?.[mode];
  return typeof f === 'number' ? f : DEFAULTS[mode];
}

function apply(px, { animate = true } = {}) {
  document.body.classList.toggle('dragging-seam', !animate);
  const h = Math.round(px);
  const collapsed = h < MIN_OPEN;
  portal.classList.toggle('collapsed', collapsed);
  portal.classList.toggle('small', h < 170);
  portal.classList.toggle('tiny', h < 100);
  document.documentElement.style.setProperty('--portal', `${collapsed ? 0 : h}px`);
  draw();
}

/** Height of the open portal in px (0 when folded). */
export function portalHeight() {
  return portal.classList.contains('collapsed') ? 0 : portal.getBoundingClientRect().height;
}

export function layout() {
  apply(fraction() * app.clientHeight, { animate: true });
}

function save(px) {
  prefs.portal = { ...(prefs.portal || {}), [mode]: px < MIN_OPEN ? 0 : px / app.clientHeight };
  savePrefs();
}

export function setSeamMode(m) {
  mode = m;
  layout();
  setTimeout(onChange, 340); // after the height transition
}

/* ---------- drawing ---------- */

function draw() {
  const r = seam.getBoundingClientRect();
  if (!r.width) return;
  const svg = $('#seam-svg');
  svg.setAttribute('viewBox', `0 0 ${r.width} ${r.height}`);
  const { lens, line } = tear(r.width, r.height, 3.4, 2.4);
  $('#seam-light').setAttribute('d', lens);
  $('#seam-core').setAttribute('d', line);
  $('#seam-glow').setAttribute('d', tear(r.width, r.height, 7, 2.4).lens);
}

/** A brief flare where a spark lands. */
export function flare() {
  seam.classList.remove('flare');
  void seam.offsetWidth;
  seam.classList.add('flare');
}

/** Shows the most important status on the seam (null clears that key). */
export function setLabel(key, text) {
  if (text) labels.set(key, text);
  else labels.delete(key);
  const el = $('#seam-label');
  const order = ['paused', 'reconnect', 'timer'];
  const top = order.find((k) => labels.has(k));
  el.hidden = !top;
  if (top) el.textContent = labels.get(top);
}

/* ---------- interaction ---------- */

export function initSeam(changed) {
  onChange = changed;
  let start = null;

  seam.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    seam.setPointerCapture(e.pointerId);
    start = { y: e.clientY, h: portalHeight(), moved: false };
  });
  seam.addEventListener('pointermove', (e) => {
    if (!start) return;
    const dy = e.clientY - start.y;
    if (!start.moved && Math.abs(dy) < 6) return;
    start.moved = true;
    apply(Math.min(Math.max(start.h + dy, 0), room()), { animate: false });
  });
  const end = () => {
    if (!start) return;
    const s = start;
    start = null;
    document.body.classList.remove('dragging-seam');
    if (!s.moved) {
      // Tap: fold the portal away, or bring it back.
      const back = s.h < MIN_OPEN ? Math.max(DEFAULTS[mode] * app.clientHeight, prefs.portalLast?.[mode] || 0) : 0;
      if (s.h >= MIN_OPEN) prefs.portalLast = { ...(prefs.portalLast || {}), [mode]: s.h };
      apply(Math.min(back, room()));
      save(back);
      haptic(10);
    } else {
      const h = portalHeight();
      apply(h);
      save(h);
      haptic(6);
    }
    setTimeout(onChange, 340);
  };
  seam.addEventListener('pointerup', end);
  seam.addEventListener('pointercancel', end);

  addEventListener('resize', () => { layout(); onChange(); });
  layout();
}
