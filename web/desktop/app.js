'use strict';

const $ = (s) => document.querySelector(s);

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

/* ------------------------------------------------- the sky (background)
 * Soft colored orbs drifting upward; phone input makes them dance. */
const canvas = $('#sky');
const ctx = canvas.getContext('2d');
const reduceMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
const COLORS = ['#8b6cff', '#4cc9f0', '#2ee6a8', '#ff6b6b', '#ffc94d'];
let W = 0, H = 0, orbs = [], surge = 0, live = false;

function resize() {
  const dpr = devicePixelRatio || 1;
  W = innerWidth;
  H = innerHeight;
  canvas.width = W * dpr;
  canvas.height = H * dpr;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  orbs = Array.from({ length: 16 }, () => newOrb(Math.random() * H));
}

function newOrb(y = H + 40) {
  return {
    x: Math.random() * W,
    y,
    r: 4 + Math.random() * 14,
    v: 0.15 + Math.random() * 0.35,
    phase: Math.random() * 6.28,
    c: COLORS[(Math.random() * COLORS.length) | 0],
  };
}

function draw() {
  ctx.clearRect(0, 0, W, H);
  const pace = (live ? 1.6 : 1) + surge * 6;
  surge *= 0.93;
  for (const o of orbs) {
    o.y -= o.v * pace;
    o.phase += 0.01 * pace;
    if (o.y < -40) Object.assign(o, newOrb());
    ctx.globalAlpha = 0.22 + surge * 0.3;
    ctx.fillStyle = o.c;
    ctx.beginPath();
    ctx.arc(o.x + Math.sin(o.phase) * 14, o.y, o.r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;
  if (!reduceMotion) requestAnimationFrame(draw);
}
addEventListener('resize', () => { resize(); if (reduceMotion) draw(); });
resize();
draw();

/* ------------------------------------------------------------- pairing */

let pairing = null;
let selectedIp = '';
let showQrWhileLive = false;
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
    const o = new Option(`${a.ip} — ${a.interface}`, a.ip);
    o.selected = a.ip === selectedIp;
    return o;
  }));
  $('#net-row').hidden = pairing.addrs.length < 2;
}

$('#net').addEventListener('change', (e) => {
  selectedIp = e.target.value;
  loadPairing();
});

$('#copy').addEventListener('click', async () => {
  if (!pairing?.url) return;
  try {
    await navigator.clipboard.writeText(pairing.url);
    $('#copy-label').textContent = 'Copied!';
    setTimeout(() => { $('#copy-label').textContent = 'Copy link'; }, 1800);
  } catch {}
});

/* -------------------------------------------------------------- status */

let last = null;
let timerTick = 0;

function ago(iso) {
  const mins = Math.max(0, Math.round((Date.now() - new Date(iso)) / 60000));
  if (mins < 1) return 'now';
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h ${mins % 60}m`;
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
  if (last && st.activity > last.activity) surge = Math.min(1, surge + 0.35);
  if (st.connected && !last?.connected) {
    showQrWhileLive = false;
    if (!settings.onboarded) {
      settings.onboarded = true;
      api('/api/settings', { method: 'POST', body: { onboarded: true } }).catch(() => {});
    }
  }
  live = st.connected;
  last = st;

  $('#pair').hidden = st.connected && !showQrWhileLive;
  $('#live').hidden = !st.connected;
  $('#warning').hidden = !st.warning;
  $('#warning').textContent = st.warning ? `⚠️ ${st.warning}` : '';
  $('#peeking').hidden = !st.peeking;

  const status = $('#status');
  const text = $('#status-text');
  if (pairing && !pairing.url && !st.connected) {
    status.dataset.state = 'off';
    text.textContent = 'Offline';
  } else if (st.paused) {
    status.dataset.state = 'paused';
    text.textContent = 'Input paused';
  } else if (st.connected) {
    status.dataset.state = 'live';
    text.textContent = 'Connected';
  } else {
    status.dataset.state = 'ready';
    text.textContent = 'Waiting for your phone';
  }

  // Returning users get a shorter welcome than first-timers.
  if (settings.onboarded && !st.connected) {
    $('#pair-title').textContent = 'Ready when you are.';
    $('#pair-sub').textContent = 'Your phone reconnects on its own. New phone? Scan the code.';
  }

  const pause = $('#pause');
  pause.textContent = st.paused ? '▶️ Resume input' : '⏸ Pause input';
  pause.classList.toggle('on', st.paused);

  if (st.connected) {
    $('#device').textContent = st.device;
    $('#since').textContent = ago(st.since);
    const rtt = $('#rtt');
    rtt.textContent = st.rttMs ? `${st.rttMs}ms` : '—';
    rtt.className = !st.rttMs ? '' : st.rttMs > 120 ? 'bad' : st.rttMs > 40 ? 'warn' : '';
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
    $('#status').dataset.state = 'off';
    $('#status-text').textContent = 'RIFT is not running';
  };
}

/* ------------------------------------------------------------- actions */

$('#pause').addEventListener('click', () => api('/api/pause', { method: 'POST', body: { paused: !last?.paused } }));
$('#disconnect').addEventListener('click', () => api('/api/disconnect', { method: 'POST' }));
$('#timer-cancel').addEventListener('click', () => api('/api/sleep', { method: 'POST', body: { minutes: 0 } }));
$('#show-qr').addEventListener('click', () => {
  showQrWhileLive = !showQrWhileLive;
  $('#show-qr').textContent = showQrWhileLive ? 'Hide pairing code' : 'Pair a different phone';
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
    btn.textContent = 'Sure? Unpairs all';
    setTimeout(() => { delete btn.dataset.armed; btn.textContent = '🔑 New pairing code'; }, 3000);
    return;
  }
  delete btn.dataset.armed;
  btn.textContent = '🔑 New pairing code';
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

// Network adapters change (Wi-Fi joins, VPN toggles): refresh the QR.
setInterval(() => { if (!live) loadPairing(); }, 15000);
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
    row.querySelector('button').textContent = ok ? '✓ On' : 'Allow';
  }
  // Keep checking while something is missing: the user flips it in Settings.
  if (p.required && !(p.input && p.screen)) setTimeout(checkPerms, 1500);
}
for (const row of document.querySelectorAll('.perm')) {
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
