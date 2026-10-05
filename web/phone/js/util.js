// Small shared helpers: DOM, storage, preferences, haptics.

export const $ = (s, root = document) => root.querySelector(s);
export const $$ = (s, root = document) => root.querySelectorAll(s);

export const enc = new TextEncoder();
export const dec = new TextDecoder();

export function store(key, value) {
  try {
    if (value === undefined) localStorage.removeItem(key);
    else localStorage.setItem(key, JSON.stringify(value));
  } catch {}
}

export function recall(key) {
  try { return JSON.parse(localStorage.getItem(key)); } catch { return null; }
}

export const prefs = Object.assign(
  {
    sens: 1.0,
    scroll: 1.0,
    natural: true,
    smart: true,
    haptics: true,
    lefty: false,
    typePeek: true,
    quality: 'balanced',
    theme: 'auto',
    view: 'type',
    peekMode: 1,
    peekZoom: 45,
    name: '',
  },
  recall('rift.prefs') || {},
);

export function savePrefs() { store('rift.prefs', prefs); }

export function haptic(ms = 8) {
  if (prefs.haptics && navigator.vibrate) navigator.vibrate(ms);
}

export function concat(...parts) {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let o = 0;
  for (const p of parts) { out.set(p, o); o += p.length; }
  return out;
}

/** What to call the computer: "PC", or "Mac" once a Mac host says hello. */
export const host = { mac: false };
export const pc = (text) => (host.mac ? text.replace(/\bPC\b/g, 'Mac') : text);

/** Tiny event emitter. */
export function emitter() {
  const handlers = {};
  return {
    on(ev, fn) { (handlers[ev] ||= []).push(fn); },
    emit(ev, ...args) { for (const fn of handlers[ev] || []) fn(...args); },
  };
}
