// RIFT on the computer: pairing, status and controls, around the tear.
import { tear } from './tear.js';

const $ = (s) => document.querySelector(s);
const $$ = (s) => document.querySelectorAll(s);

/* ---------------------------------------------------------------- auth
 * The admin key arrives in the URL fragment when RIFT opens this window.
 * Keep it for reloads, then scrub it from the address bar. */
let adminKey = location.hash.slice(1);
try {
  if (adminKey) sessionStorage.setItem('rift.admin', adminKey);
  else adminKey = sessionStorage.getItem('rift.admin') || '';
} catch {}
history.replaceState(null, '', '/');

async function api(path, { method = 'GET', body } = {}) {
  const res = await fetch(path, {
    method,
    headers: { 'X-Rift-Admin': adminKey, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`${path}: ${res.status}`);
  return res.status === 204 ? null : res.json();
}

/* ------------------------------------------------------------ the tear
 * Drawn vertically: the horizontal tear generator, rotated a quarter turn.
 * It opens wider while a phone is reaching through. */
const rift = $('#rift');
let opening = 24;
let openTarget = 24;
let raf = 0;

function drawTear() {
  const r = rift.getBoundingClientRect();
  if (!r.width) return;
  const svg = $('#tear-svg');
  svg.setAttribute('viewBox', `0 0 ${r.width} ${r.height}`);
  // Build along the height, then rotate into place around the centre.
  const rot = `rotate(90 ${r.width / 2} ${r.height / 2}) translate(${(r.width - r.height) / 2} ${(r.height - r.width) / 2})`;
  const t = tear(r.height, r.width, opening, 9, 5);
  const glow = tear(r.height, r.width, opening * 2.1, 9, 5);
  for (const [id, d] of [['#tear-body', t.lens], ['#tear-core', t.line], ['#tear-glow', glow.lens]]) {
    $(id).setAttribute('d', d);
    $(id).setAttribute('transform', rot);
  }
}

function animateTear() {
  opening += (openTarget - opening) * 0.08;
  drawTear();
  raf = Math.abs(openTarget - opening) > 0.2 ? requestAnimationFrame(animateTear) : 0;
}

function setOpening(px) {
  openTarget = px;
  if (!raf) raf = requestAnimationFrame(animateTear);
}

/* ------------------------------------------------------------- sparks
 * Each input from the phone arrives as light falling through the tear. */
const canvas = $('#sparks');
const ctx = canvas.getContext('2d');
const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
const COLORS = ['#ffd25e', '#ff9a4d', '#ff7a3d', '#ff5a6a', '#ff3d7f'];
let sparks = [];
let sraf = 0;

function sizeCanvas() {
  const r = rift.getBoundingClientRect();
  const dpr = Math.min(devicePixelRatio || 1, 2);
  canvas.width = r.width * dpr;
  canvas.height = r.height * dpr;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function rain(n) {
  if (reduceMotion || document.hidden) return;
  const r = rift.getBoundingClientRect();
  for (let i = 0; i < n; i++) {
    sparks.push({
      x: r.width / 2 + (Math.random() - 0.5) * 50,
      y: -10 - Math.random() * 60,
      vy: 4 + Math.random() * 5,
      sway: Math.random() * 6.28,
      r: 1.2 + Math.random() * 2,
      c: COLORS[(Math.random() * COLORS.length) | 0],
    });
  }
  if (sparks.length > 220) sparks = sparks.slice(-220);
  if (!sraf) sraf = requestAnimationFrame(stepSparks);
}

function stepSparks() {
  const r = rift.getBoundingClientRect();
  ctx.clearRect(0, 0, r.width, r.height);
  for (const s of sparks) {
    s.y += s.vy;
    s.sway += 0.1;
    const x = s.x + Math.sin(s.sway) * 6;
    ctx.globalAlpha = Math.max(0, 1 - s.y / r.height);
    ctx.fillStyle = s.c;
    ctx.shadowBlur = 10;
    ctx.shadowColor = s.c;
    ctx.beginPath();
    ctx.arc(x, s.y, s.r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;
  ctx.shadowBlur = 0;
  sparks = sparks.filter((s) => s.y < r.height + 10);
  sraf = sparks.length ? requestAnimationFrame(stepSparks) : 0;
  if (!sraf) ctx.clearRect(0, 0, r.width, r.height);
}

function layoutRift() {
  sizeCanvas();
  drawTear();
}
addEventListener('resize', layoutRift);
layoutRift();

/* ------------------------------------------------------------- pairing */

let pairing = null;
let selectedIp = '';
let forceQr = false;
let settings = { peekAllowed: true, onboarded: false };

async function loadPairing() {
  try {
    pairing = await api(`/api/pairing?ip=${encodeURIComponent(selectedIp)}`);
  } catch {
    return;
  }
  selectedIp = pairing.ip || '';
  $('#offline').hidden = !!pairing.url;
  if (pairing.qr) $('#qr').src = pairing.qr;
  else $('#qr').removeAttribute('src');
  $('#link').textContent = pairing.url || '';
  $('#copy').hidden = !pairing.url;
  const sel = $('#net');
  sel.replaceChildren(...pairing.addrs.map((a) => {
    const o = new Option(`${a.ip} · ${a.interface}`, a.ip);
    o.selected = a.ip === selectedIp;
    return o;
  }));
  $('#net-row').hidden = pairing.addrs.length < 2;
  if (last) render(last);
}

$('#net').addEventListener('change', (e) => {
  selectedIp = e.target.value;
  loadPairing();
});

$('#copy').addEventListener('click', async () => {
  if (!pairing?.url) return;
  try {
    await navigator.clipboard.writeText(pairing.url);
    $('#copy-label').textContent = 'copied';
    setTimeout(() => { $('#copy-label').textContent = 'copy'; }, 1800);
  } catch {}
});

/* -------------------------------------------------------------- status */

let last = null;
let timerTick = 0;

function ago(iso) {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso)) / 60000));
  if (mins < 1) return 'now';
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h${String(mins % 60).padStart(2, '0')}`;
}

function fmtLeft(iso) {
  const s = Math.max(0, Math.round((new Date(iso) - Date.now()) / 1000));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}` : `${m}:${String(r).padStart(2, '0')}`;
}

function renderTimer() {
  const at = last?.sleepAt;
  $('#timer').hidden = !at;
  if (at) $('#timer-left').textContent = fmtLeft(at);
}

function render(st) {
  if (last && st.activity > last.activity) rain(Math.min(3 + (st.activity - last.activity) * 2, 14));
  if (st.connected && !last?.connected) {
    forceQr = false;
    if (!settings.onboarded) {
      settings.onboarded = true;
      api('/api/settings', { method: 'POST', body: { onboarded: true } }).catch(() => {});
    }
  }
  last = st;

  const offline = pairing && !pairing.url && !st.connected;
  const state = offline ? 'off' : st.connected ? (st.paused ? 'frozen' : 'live') : 'ready';
  document.body.dataset.state = state;
  $('#status-text').textContent = { off: 'offline', frozen: 'input frozen', live: 'rift open', ready: 'waiting for a phone' }[state];
  setOpening(st.connected ? 38 : 24);

  $('#pair').hidden = st.connected && !forceQr;
  $('#live').hidden = !st.connected;
  $('#qr-card').classList.toggle('forced', forceQr);
  $('#warning').hidden = !st.warning;
  $('#warning').textContent = st.warning || '';
  $('#peeking').hidden = !st.peeking;

  // Returning users get a shorter welcome than first-timers.
  if (settings.onboarded && !st.connected) {
    $('#pair-kicker').textContent = 'ready when you are';
    $('#pair-title').textContent = 'The rift is waiting.';
    $('#pair-sub').textContent = macify('Your phone reopens it on its own. A new phone? Scan the code.');
  }

  const pause = $('#pause');
  pause.textContent = st.paused ? 'Thaw input' : 'Freeze input';
  pause.classList.toggle('on', st.paused);

  if (st.connected) {
    $('#device').textContent = st.device;
    $('#since').textContent = ago(st.since);
    const rtt = $('#rtt');
    rtt.textContent = st.rttMs ? `${st.rttMs}ms` : '—';
    rtt.className = !st.rttMs ? '' : st.rttMs > 150 ? 'bad' : st.rttMs > 60 ? 'warn' : '';
    $('#events').textContent = st.activity > 9999 ? `${(st.activity / 1000).toFixed(1)}k` : st.activity.toLocaleString();
  }

  clearInterval(timerTick);
  if (st.sleepAt) timerTick = setInterval(renderTimer, 1000);
  renderTimer();
}

function listen() {
  const es = new EventSource(`/api/events?k=${encodeURIComponent(adminKey)}`);
  es.onmessage = (e) => render(JSON.parse(e.data));
  es.onerror = () => {
    document.body.dataset.state = 'off';
    $('#status-text').textContent = 'RIFT is not running';
  };
}

/* ------------------------------------------------------------- actions */

$('#pause').addEventListener('click', () => api('/api/pause', { method: 'POST', body: { paused: !last?.paused } }));
$('#disconnect').addEventListener('click', () => api('/api/disconnect', { method: 'POST' }));
$('#timer-cancel').addEventListener('click', () => api('/api/sleep', { method: 'POST', body: { minutes: 0 } }));
$('#show-qr').addEventListener('click', () => {
  forceQr = !forceQr;
  $('#show-qr').textContent = forceQr ? 'Hide the code' : 'Open a rift to another phone';
  if (last) render(last);
});

const allowPeek = $('#allow-peek');
async function setPeek(on) {
  try {
    settings = await api('/api/settings', { method: 'POST', body: { peekAllowed: on } });
  } catch {}
  allowPeek.checked = settings.peekAllowed;
}
allowPeek.addEventListener('change', () => setPeek(allowPeek.checked));
$('#stop-peek').addEventListener('click', () => setPeek(false));

$('#rotate').addEventListener('click', async (e) => {
  const btn = e.currentTarget;
  if (!btn.dataset.armed) {
    btn.dataset.armed = '1';
    btn.textContent = 'Sure? Every phone rescans';
    setTimeout(() => { delete btn.dataset.armed; btn.textContent = 'Seal every rift'; }, 3000);
    return;
  }
  delete btn.dataset.armed;
  btn.textContent = 'Seal every rift';
  await api('/api/pairing/reset', { method: 'POST' });
  loadPairing();
});

$('#quit').addEventListener('click', async () => {
  await api('/api/quit', { method: 'POST' }).catch(() => {});
  window.close();
});

const autostart = $('#autostart');
api('/api/autostart').then((r) => { autostart.checked = r.enabled; }).catch(() => {});
autostart.addEventListener('change', async () => {
  try {
    const r = await api('/api/autostart', { method: 'POST', body: { enabled: autostart.checked } });
    autostart.checked = r.enabled;
  } catch {
    autostart.checked = !autostart.checked;
  }
});

// Network adapters change (Wi-Fi joins, VPN toggles): refresh the code.
setInterval(() => { if (!last?.connected) loadPairing(); }, 15000);
addEventListener('focus', loadPairing);

/* ------------------------------------------- macOS permission checklist */

let isMac = false;
const macify = (t) => (isMac ? t.replace(/\bPC\b/g, 'Mac') : t);

async function checkPerms() {
  let p;
  try { p = await api('/api/permissions'); } catch { return; }
  if (p.required && !isMac) {
    // Only macOS asks for permissions, so this is a Mac: say so in the copy.
    isMac = true;
    const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let n = walk.nextNode(); n; n = walk.nextNode()) n.nodeValue = macify(n.nodeValue);
  }
  const box = $('#perms');
  box.hidden = !p.required || (p.input && p.screen);
  for (const row of box.querySelectorAll('.perm')) {
    const ok = p[row.dataset.kind];
    row.classList.toggle('ok', ok);
    row.querySelector('button').textContent = ok ? 'on' : 'Allow';
  }
  // Keep checking while something is missing: the user flips it in Settings.
  if (p.required && !(p.input && p.screen)) setTimeout(checkPerms, 1500);
}
for (const row of $$('.perm')) {
  row.querySelector('button').addEventListener('click', () =>
    api('/api/permissions', { method: 'POST', body: { kind: row.dataset.kind } }).then(checkPerms).catch(() => {}));
}

checkPerms();
api('/api/settings').then((s) => {
  settings = s;
  allowPeek.checked = s.peekAllowed;
  if (last) render(last);
}).catch(() => {});
loadPairing();
listen();
