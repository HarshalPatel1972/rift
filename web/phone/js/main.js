// RIFT phone app: modes, the rift's states, and wiring.
import { $, $$, haptic, pc, prefs, savePrefs } from './util.js';
import { FLAG, connect, forgetPairing, hasKey, link, loadPairingKey, resume, useKey } from './link.js';
import { applyHostOS, bindKeys } from './press.js';
import { clearMods, editor, initTyping, latchedMods } from './typing.js';
import { initTrackpad } from './trackpad.js';
import { initSeam, setLabel, setSeamMode } from './seam.js';
import { initPortal, portalState, refreshPortal, setPortalMode } from './portal.js';
import { initSparks } from './sparks.js';
import { initChill } from './chill.js';
import { initStory, showStory, storySeen } from './story.js';
import { initSettings } from './settings.js';

const MODES = ['type', 'touch', 'media', 'keys'];

/* ---------------------------------------------------------------- modes */

function setMode(mode, { focus = true } = {}) {
  if (!MODES.includes(mode)) mode = 'type';
  prefs.mode = mode;
  savePrefs();
  document.body.dataset.mode = mode;
  $$('.modes button').forEach((b) => {
    if (b.dataset.mode === mode) b.setAttribute('aria-current', 'page');
    else b.removeAttribute('aria-current');
  });
  if (mode === 'type' && focus) editor.focus();
  else editor.blur();
  setSeamMode(mode);
  setPortalMode(mode);
}

// Switch on pointerup rather than waiting for a synthesized click, which
// browsers sometimes drop after complex touch gestures; click stays for
// keyboards and screen readers.
$$('.modes button').forEach((b) => {
  const go = () => {
    if (document.body.dataset.mode === b.dataset.mode) return;
    haptic(6);
    setMode(b.dataset.mode);
  };
  b.addEventListener('pointerup', (e) => { if (e.isPrimary) go(); });
  b.addEventListener('click', go);
});

// Hide the mode bar while the soft keyboard is up, to give typing the room.
if (window.visualViewport) {
  let tallest = visualViewport.height;
  const check = () => {
    tallest = Math.max(tallest, visualViewport.height);
    document.body.classList.toggle('kb', visualViewport.height < tallest * 0.75 && document.activeElement === editor);
  };
  visualViewport.addEventListener('resize', check);
  addEventListener('orientationchange', () => { tallest = 0; setTimeout(check, 300); });
  editor.addEventListener('blur', () => document.body.classList.remove('kb'));
  editor.addEventListener('focus', () => setTimeout(check, 350));
}

/* ------------------------------------------------- the rift's condition
 * body[data-link] drives the seam's light:
 *   connected · weak (slow link) · connecting · paused (frozen) · offline */

let state = 'connecting';
let rtt = 0;
let paused = false;

function renderLink() {
  let s = state;
  if (s === 'connected' && paused) s = 'paused';
  else if (s === 'connected' && rtt > 150) s = 'weak';
  document.body.dataset.link = s;
  setLabel('paused', paused && state === 'connected' ? 'input paused' : null);
  setLabel('reconnect', state === 'connecting' && link.host ? 'reopening…' : null);
  $('#where-label').textContent = {
    connected: 'rift open to', weak: 'rift open to', paused: 'rift frozen at', connecting: 'reaching', offline: 'rift closed',
  }[s];
}

link.on('status', (st, text) => {
  state = st === 'connected' ? 'connected' : st === 'connecting' ? 'connecting' : 'offline';
  if (st === 'connected') $('#host').textContent = text;
  else if (!link.host) $('#host').textContent = text;
  if (st !== 'connected') $('#rtt').hidden = true;
  renderLink();
});

link.on('ping', ({ rtt: r, flags }) => {
  paused = !!(flags & FLAG.PAUSED);
  applyHostOS(!!(flags & FLAG.MAC));
  if (r) {
    rtt = r;
    const el = $('#rtt');
    el.hidden = false;
    el.textContent = `${r}ms`;
    el.className = 'rtt' + (r > 150 ? ' bad' : r > 60 ? ' warn' : '');
  }
  renderLink();
});

/* --------------------------------------------- sealed: no open rift */

const SEALED = {
  nokey: {
    title: 'The rift is closed.',
    text: 'Open RIFT on your computer and scan its code with your camera. That opens it.',
  },
  unpaired: {
    title: 'This rift was sealed.',
    text: 'Your PC made a new pairing code. Scan it again to reopen the rift.',
  },
  replaced: {
    title: 'Another phone reached through.',
    text: 'Someone else opened a rift to your PC, so this one closed.',
    action: 'Take it back',
  },
  kicked: {
    title: 'Closed from the other side.',
    text: 'Your PC ended this session.',
    action: 'Reopen the rift',
  },
};

function seal(kind) {
  const o = SEALED[kind];
  $('#sealed-title').textContent = pc(o.title);
  $('#sealed-text').textContent = pc(o.text);
  const btn = $('#sealed-action');
  btn.hidden = !o.action;
  btn.textContent = o.action || '';
  $('#sealed').hidden = false;
  editor.blur();
}

$('#sealed-action').addEventListener('click', () => {
  $('#sealed').hidden = true;
  resume();
});

link.on('stopped', seal);
link.on('ready', () => {
  $('#sealed').hidden = true;
  if (!storySeen()) showStory(() => setMode(prefs.mode));
});

/* ----------------------------------------------------------------- boot */

initTyping();
initTrackpad();
initSeam(refreshPortal);
initPortal();
initSparks();
initChill();
initStory();
initSettings({
  onForget() {
    forgetPairing();
    seal('nokey');
  },
  onReplay() {
    showStory(() => setMode(prefs.mode));
  },
});
bindKeys(
  (el) => (el.closest('#keyrow') ? latchedMods() : 0),
  (el) => { if (el.closest('#keyrow') && latchedMods()) clearMods(); },
);

setMode(MODES.includes(prefs.mode) ? prefs.mode : 'type', { focus: false });
renderLink();

const pairing = loadPairingKey();
if (!pairing) {
  link.emit('status', 'offline', 'not paired');
  seal('nokey');
} else {
  useKey(pairing);
  connect();
}

// Exposed for debugging from the console.
window.rift = { link, hasKey, portal: portalState };
