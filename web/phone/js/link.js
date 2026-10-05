/* The encrypted link to the PC.
 *
 * Wire format (see internal/protocol): every frame after the server's
 * plaintext challenge is nonce[24] || secretbox(seq u32 LE || op u8 || payload),
 * with one derived key per direction. The pairing key arrives in the URL
 * fragment (never sent over the network) and is kept in localStorage.
 *
 * Events: status(state, text) · ready(host) · ping({rtt, flags, timer})
 *         frame(payload) · stopped(kind)
 */
import { concat, dec, emitter, enc, prefs, recall, store } from './util.js';

export const OP = {
  HELLO: 1, EDIT: 2, KEY: 3, MOVE: 4, BUTTON: 5, SCROLL: 6, PONG: 7,
  ACTION: 8, PEEK: 9, ACK: 10, POINT: 11,
  WELCOME: 0x81, PING: 0x82, FRAME: 0x83,
};
export const FLAG = { PAUSED: 1, PEEK: 2, POWER: 4, MAC: 8 };
export const BTN = { UP: 0, DOWN: 1, CLICK: 2 };
export const PEEK = { OFF: 0, SCREEN: 1, CURSOR: 2, CARET: 3 };
export const ACTION = { LOCK: 1, DISPLAY_OFF: 2, SLEEP: 3, SLEEP_TIMER: 4 };
const CLOSE = { UNPAIRED: 4401, REPLACED: 4409, KICKED: 4410 };

export const link = Object.assign(emitter(), {
  ready: false,
  host: '',
  flags: 0,
});

const conn = {
  ws: null,
  phase: 'idle', // idle | challenge | welcome | ready | stopped
  keys: null,
  seqOut: 0,
  seqIn: 0,
  nonce: null,
  retry: 400,
  retryTimer: 0,
  lastPing: 0,
};

/* ------------------------------------------------------------- pairing */

function b64urlDecode(s) {
  const b = atob(s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4));
  return Uint8Array.from(b, (c) => c.charCodeAt(0));
}

function equalBytes(a, b) {
  if (a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a[i] ^ b[i];
  return d === 0;
}

/** Reads the key from the URL fragment (then scrubs it) or from storage. */
export function loadPairingKey() {
  const m = location.hash.match(/[#&]k=([A-Za-z0-9_-]+)/);
  if (m) {
    try {
      if (b64urlDecode(m[1]).length === 32) store('rift.key', m[1]);
    } catch {}
    history.replaceState(null, '', location.pathname);
  }
  const saved = recall('rift.key');
  if (!saved) return null;
  try {
    const k = b64urlDecode(saved);
    return k.length === 32 ? k : null;
  } catch { return null; }
}

export function forgetPairing() {
  store('rift.key', undefined);
  conn.keys = null;
  conn.phase = 'stopped';
  conn.ws?.close();
}

function deriveKey(pairing, label) {
  return nacl.hash(concat(enc.encode(label), pairing)).subarray(0, 32);
}

export function useKey(pairing) {
  conn.keys = { c2s: deriveKey(pairing, 'rift/c2s/v1'), s2c: deriveKey(pairing, 'rift/s2c/v1') };
}

export function hasKey() { return !!conn.keys; }

export function deviceName() {
  if (prefs.name) return prefs.name;
  const ua = navigator.userAgent;
  if (/iPad/.test(ua)) return 'iPad';
  if (/iPhone/.test(ua)) return 'iPhone';
  const m = ua.match(/Android [^;)]*; ([^;)]+)(?: Build|\))/);
  if (m && m[1].trim() !== 'K') return m[1].trim();
  if (/Android/.test(ua)) return 'Android phone';
  return 'Browser';
}

/* ---------------------------------------------------------- connection */

export function connect() {
  clearTimeout(conn.retryTimer);
  if (!conn.keys || conn.phase === 'stopped') return;
  if (conn.ws && conn.ws.readyState <= 1) return;

  link.emit('status', 'connecting', 'Connecting…');
  const ws = new WebSocket(`ws://${location.host}/ws`);
  ws.binaryType = 'arraybuffer';
  conn.ws = ws;
  conn.phase = 'challenge';

  ws.onmessage = (e) => {
    if (ws !== conn.ws) return;
    const data = new Uint8Array(e.data);
    if (conn.phase === 'challenge') return onChallenge(data);
    const msg = openFrame(data);
    if (!msg) { ws.close(); return; }
    onMessage(msg.op, msg.payload);
  };

  ws.onclose = (e) => {
    if (ws !== conn.ws) return;
    conn.ws = null;
    setReady(false);
    if (conn.phase === 'stopped') return;
    conn.phase = 'idle';
    if (e.code === CLOSE.UNPAIRED) return stop('unpaired');
    if (e.code === CLOSE.REPLACED) return stop('replaced');
    if (e.code === CLOSE.KICKED) return stop('kicked');
    link.emit('status', 'offline', navigator.onLine ? 'Reconnecting…' : 'Offline');
    conn.retryTimer = setTimeout(connect, conn.retry * (0.8 + Math.random() * 0.4));
    conn.retry = Math.min(conn.retry * 1.6, 5000);
  };
}

/** Restart after a stop (e.g. "Take back control"). */
export function resume() {
  conn.phase = 'idle';
  conn.retry = 400;
  connect();
}

function stop(kind) {
  conn.phase = 'stopped';
  link.emit('status', 'offline', 'Not connected');
  link.emit('stopped', kind);
}

function setReady(r) {
  if (link.ready === r) return;
  link.ready = r;
  if (r) link.emit('ready', link.host);
}

function onChallenge(data) {
  if (data.length !== 19 || data[0] !== 0x52 || data[1] !== 0x46 || data[2] !== 1) {
    conn.ws.close();
    return;
  }
  conn.seqOut = 0;
  conn.seqIn = 0;
  conn.nonce = nacl.randomBytes(16);
  conn.phase = 'welcome';
  sendFrame(OP.HELLO, concat(data.subarray(3), conn.nonce, enc.encode(deviceName())));
}

function onMessage(op, p) {
  if (conn.phase === 'welcome') {
    if (op !== OP.WELCOME || p.length < 17 || !equalBytes(p.subarray(0, 16), conn.nonce)) {
      conn.ws.close();
      return;
    }
    conn.phase = 'ready';
    conn.retry = 400;
    conn.lastPing = performance.now();
    link.host = dec.decode(p.subarray(17)) || 'your PC';
    link.flags = p[16];
    link.emit('status', 'connected', link.host);
    link.emit('ping', { rtt: 0, flags: link.flags, timer: 0 });
    setReady(true);
    return;
  }
  if (op === OP.PING && p.length >= 11) {
    conn.lastPing = performance.now();
    sendFrame(OP.PONG, p.subarray(0, 8));
    const v = new DataView(p.buffer, p.byteOffset, p.byteLength);
    link.flags = p[10];
    link.emit('ping', {
      rtt: v.getUint16(8, true),
      flags: p[10],
      timer: p.length >= 15 ? v.getUint32(11, true) : 0,
    });
  } else if (op === OP.FRAME && p.length > 12) {
    link.emit('frame', p);
  }
}

function sealFrame(op, payload) {
  const pt = new Uint8Array(5 + payload.length);
  new DataView(pt.buffer).setUint32(0, ++conn.seqOut, true);
  pt[4] = op;
  pt.set(payload, 5);
  const nonce = nacl.randomBytes(24);
  return concat(nonce, nacl.secretbox(pt, nonce, conn.keys.c2s));
}

function openFrame(frame) {
  if (frame.length < 24 + 16 + 5) return null;
  const pt = nacl.secretbox.open(frame.subarray(24), frame.subarray(0, 24), conn.keys.s2c);
  if (!pt) return null;
  const seq = new DataView(pt.buffer, pt.byteOffset).getUint32(0, true);
  if (seq <= conn.seqIn) return null;
  conn.seqIn = seq;
  return { op: pt[4], payload: pt.subarray(5) };
}

function sendFrame(op, payload = new Uint8Array(0)) {
  conn.ws.send(sealFrame(op, payload));
}

/** Sends if connected; returns false when dropped. */
export function send(op, payload) {
  if (conn.phase !== 'ready' || !conn.ws) return false;
  // Under backpressure, drop pointer motion rather than queueing stale moves.
  if ((op === OP.MOVE || op === OP.POINT) && conn.ws.bufferedAmount > 64 * 1024) return false;
  sendFrame(op, payload);
  return true;
}

/* ------------------------------------------------------------ helpers */

const clamp16 = (v) => Math.max(-32768, Math.min(32767, v));

export function i16pair(a, b) {
  const p = new Uint8Array(4);
  const v = new DataView(p.buffer);
  v.setInt16(0, clamp16(a), true);
  v.setInt16(2, clamp16(b), true);
  return p;
}

export const sendKey = (vk, mods = 0) => send(OP.KEY, Uint8Array.of(vk, mods));
export const sendButton = (button, action) => send(OP.BUTTON, Uint8Array.of(button, action));
export const sendMove = (dx, dy) => send(OP.MOVE, i16pair(dx, dy));
export const sendScroll = (dx, dy) => send(OP.SCROLL, i16pair(dx, dy));

export function sendEdit(del, text) {
  // Keep frames well under the server's 64 KiB limit.
  const bytes = enc.encode(text);
  const CHUNK = 16000;
  let off = 0;
  do {
    let end = Math.min(off + CHUNK, bytes.length);
    while (end < bytes.length && (bytes[end] & 0xc0) === 0x80) end--; // don't split UTF-8
    const p = new Uint8Array(2 + end - off);
    new DataView(p.buffer).setUint16(0, Math.min(del, 65535), true);
    p.set(bytes.subarray(off, end), 2);
    send(OP.EDIT, p);
    del = 0;
    off = end;
  } while (off < bytes.length);
}

export function sendAction(action, arg = 0) {
  const p = new Uint8Array(5);
  p[0] = action;
  new DataView(p.buffer).setUint32(1, arg, true);
  return send(OP.ACTION, p);
}

export function sendPeek(mode, maxWidth, span, aspect) {
  const p = new Uint8Array(7);
  const v = new DataView(p.buffer);
  p[0] = mode;
  v.setUint16(1, maxWidth, true);
  v.setUint16(3, span, true);
  v.setUint16(5, aspect, true);
  return send(OP.PEEK, p);
}

export function sendAck(frameId) {
  const p = new Uint8Array(4);
  new DataView(p.buffer).setUint32(0, frameId, true);
  return send(OP.ACK, p);
}

/** Absolute point on the last frame; x, y in 0..1. click: 0 none, 1 left, 2 double, 3 right. */
export function sendPoint(x, y, click) {
  const p = new Uint8Array(5);
  const v = new DataView(p.buffer);
  v.setUint16(0, Math.round(Math.max(0, Math.min(1, x)) * 65535), true);
  v.setUint16(2, Math.round(Math.max(0, Math.min(1, y)) * 65535), true);
  p[4] = click;
  return send(OP.POINT, p);
}

/* ------------------------------------------------------------- upkeep */

// The server pings every 2 s; silence means a dead link (e.g. Wi-Fi handoff)
// that the OS hasn't noticed yet.
setInterval(() => {
  if (conn.phase === 'ready' && performance.now() - conn.lastPing > 7000) conn.ws?.close();
}, 1000);

document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible') {
    conn.retry = 400;
    connect();
  }
});
addEventListener('online', () => { conn.retry = 400; connect(); });
