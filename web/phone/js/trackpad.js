/* Trackpad gestures (Pointer Events):
 *   1 finger move           → cursor, with velocity-based acceleration
 *   1 finger tap            → left click
 *   double-tap, hold & move → drag (button pressed on the second touch)
 *   2 fingers move          → scroll, with momentum after lift
 *   2 / 3 finger tap        → right / middle click
 * Plus a one-thumb scroll strip and hold-able hardware-style buttons.
 * Motion is accumulated with sub-pixel remainders and flushed once per frame.
 */
import { $, $$, haptic, prefs } from './util.js';
import { BTN, sendButton, sendMove, sendScroll } from './link.js';

const TAP_MS = 260;
const TAP_SLOP = 9;
const WHEEL_PER_PX = 4; // 120 wheel units ≈ 30 px of finger travel

const move = { x: 0, y: 0 };
const wheel = { x: 0, y: 0 };
let momentum = null;
let frame = 0;

function schedule() {
  if (!frame) frame = requestAnimationFrame(flush);
}

function flush() {
  frame = 0;
  if (momentum) {
    addScroll(momentum.vx * 16, momentum.vy * 16);
    momentum.vx *= 0.94;
    momentum.vy *= 0.94;
    if (Math.hypot(momentum.vx, momentum.vy) < 0.02) momentum = null;
  }
  const mx = Math.trunc(move.x), my = Math.trunc(move.y);
  if (mx || my) {
    move.x -= mx;
    move.y -= my;
    sendMove(mx, my);
  }
  const wx = Math.trunc(wheel.x), wy = Math.trunc(wheel.y);
  if (wx || wy) {
    wheel.x -= wx;
    wheel.y -= wy;
    sendScroll(wx, wy);
  }
  if (momentum) schedule();
}

/** Finger travel (CSS px) → wheel units. Windows: +wheel scrolls up and
 * +hwheel scrolls right; "natural" makes content follow the finger. */
function addScroll(dx, dy) {
  const k = WHEEL_PER_PX * prefs.scroll * (prefs.natural ? 1 : -1);
  wheel.x -= dx * k;
  wheel.y += dy * k;
}

function accel(speed) {
  // speed in CSS px/ms: precise when slow, ~4x when flicking.
  return prefs.sens * Math.min(1.1 + 2.4 * Math.max(0, speed - 0.05), 5);
}

/** Turns el into a trackpad. */
export function attachTrackpad(el) {
  const pointers = new Map();
  let gesture = null;
  let lastTap = { t: 0, x: 0, y: 0 };
  let dragging = false;

  const glow = (e) => {
    const r = el.getBoundingClientRect();
    el.style.setProperty('--x', `${e.clientX - r.left}px`);
    el.style.setProperty('--y', `${e.clientY - r.top}px`);
  };

  el.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY, sx: e.clientX, sy: e.clientY, t: e.timeStamp });
    momentum = null;
    el.classList.add('touching', 'used');
    glow(e);

    if (pointers.size === 1) {
      gesture = { start: e.timeStamp, max: 1, moved: false, vx: 0, vy: 0, lastT: e.timeStamp };
      // Tap-and-a-half: a touch right after a tap presses the button at once,
      // so lifting gives a double-click and moving drags from the exact spot.
      if (e.timeStamp - lastTap.t < 300 && Math.hypot(e.clientX - lastTap.x, e.clientY - lastTap.y) < 40) {
        dragging = true;
        lastTap.t = 0;
        sendButton(0, BTN.DOWN);
      }
    } else if (gesture) {
      gesture.max = Math.max(gesture.max, pointers.size);
    }
  });

  el.addEventListener('pointermove', (e) => {
    const p = pointers.get(e.pointerId);
    if (!p || !gesture) return;
    const dx = e.clientX - p.x;
    const dy = e.clientY - p.y;
    // Floor at ~one 120 Hz frame so a burst of events can't fake a fast flick.
    const dt = Math.max(e.timeStamp - p.t, 8);
    p.x = e.clientX;
    p.y = e.clientY;
    p.t = e.timeStamp;
    if (!gesture.moved && Math.hypot(e.clientX - p.sx, e.clientY - p.sy) > TAP_SLOP) gesture.moved = true;
    if (pointers.size === 1) glow(e);

    if (pointers.size === 1 && gesture.max === 1) {
      if (dragging && gesture.moved && !el.classList.contains('dragging')) {
        el.classList.add('dragging');
        haptic(15);
      }
      const g = accel(Math.hypot(dx, dy) / dt);
      move.x += dx * g;
      move.y += dy * g;
      schedule();
    } else if (pointers.size === 2) {
      // Each finger contributes half, so the sum is the average motion.
      const sx = dx / 2, sy = dy / 2;
      addScroll(sx, sy);
      const k = 0.25; // smoothed velocity for momentum
      gesture.vx = gesture.vx * (1 - k) + (sx * 2 / dt) * k;
      gesture.vy = gesture.vy * (1 - k) + (sy * 2 / dt) * k;
      gesture.lastT = e.timeStamp;
      schedule();
    }
  });

  function end(e) {
    if (!pointers.delete(e.pointerId) || pointers.size || !gesture) return;
    el.classList.remove('touching');
    const g = gesture;
    gesture = null;

    if (dragging) {
      dragging = false;
      el.classList.remove('dragging');
      sendButton(0, BTN.UP);
      if (!g.moved) haptic(8); // completed a double-click
      return;
    }
    const quick = e.timeStamp - g.start < TAP_MS;
    if (!g.moved && quick && e.type === 'pointerup') {
      const button = [0, 0, 1, 2][Math.min(g.max, 3)];
      sendButton(button, BTN.CLICK);
      haptic(g.max > 1 ? 14 : 8);
      lastTap = g.max === 1 ? { t: e.timeStamp, x: e.clientX, y: e.clientY } : { t: 0, x: 0, y: 0 };
      return;
    }
    if (g.max === 2 && g.moved && e.timeStamp - g.lastT < 60 && Math.hypot(g.vx, g.vy) > 0.25) {
      momentum = { vx: g.vx, vy: g.vy };
      schedule();
    }
  }
  el.addEventListener('pointerup', end);
  el.addEventListener('pointercancel', end);
}

/** A vertical strip you scroll with one thumb, with momentum. */
function attachStrip(el) {
  let last = null;
  let v = 0;
  el.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    el.setPointerCapture(e.pointerId);
    momentum = null;
    last = { y: e.clientY, t: e.timeStamp };
    v = 0;
    el.classList.add('active');
    haptic(5);
  });
  el.addEventListener('pointermove', (e) => {
    if (!last) return;
    const dy = e.clientY - last.y;
    const dt = Math.max(e.timeStamp - last.t, 8);
    last = { y: e.clientY, t: e.timeStamp };
    addScroll(0, dy * 1.4);
    v = v * 0.75 + (dy * 1.4 / dt) * 0.25;
    schedule();
  });
  const end = (e) => {
    if (!last) return;
    el.classList.remove('active');
    if (e.timeStamp - last.t < 60 && Math.abs(v) > 0.25) {
      momentum = { vx: 0, vy: v };
      schedule();
    }
    last = null;
  };
  el.addEventListener('pointerup', end);
  el.addEventListener('pointercancel', end);
}

// Hardware-style buttons: press = down, release = up, so you can hold Left
// with your thumb while dragging on the pad with a finger.
function attachButtons() {
  $$('[data-button]').forEach((el) => {
    const b = Number(el.dataset.button);
    let held = false;
    const up = () => {
      if (!held) return;
      held = false;
      el.classList.remove('held');
      sendButton(b, BTN.UP);
    };
    el.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      el.setPointerCapture(e.pointerId);
      held = true;
      el.classList.add('held');
      haptic();
      sendButton(b, BTN.DOWN);
    });
    el.addEventListener('pointerup', up);
    el.addEventListener('pointercancel', up);
    el.addEventListener('contextmenu', (e) => e.preventDefault());
  });
}

export function initTrackpad() {
  attachTrackpad($('#pad'));
  attachTrackpad($('#peek-pad'));
  attachStrip($('#strip'));
  attachButtons();
}
