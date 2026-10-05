// RIFT phone app: wiring, views and connection states.
import { $, $$, haptic, prefs, savePrefs } from './util.js';
import { FLAG, connect, forgetPairing, hasKey, link, loadPairingKey, resume, useKey } from './link.js';
import { bindKeys } from './press.js';
import { clearMods, editor, initTyping, latchedMods } from './typing.js';
import { initTrackpad } from './trackpad.js';
import { initPeek, peekState, setPeekView } from './peek.js';
import { initChill } from './chill.js';
import { initStory, showStory, storySeen } from './story.js';
import { initSettings } from './settings.js';

const VIEWS = ['type', 'pad', 'peek', 'chill', 'keys'];

/* ---------------------------------------------------------------- views */

function setView(view, { focus = true } = {}) {
  if (!VIEWS.includes(view)) view = 'type';
  prefs.view = view;
  savePrefs();
  document.body.dataset.view = view;
  $$('.dock button').forEach((b) => b.toggleAttribute('aria-current', b.dataset.view === view));
  $$('.dock button[aria-current]').forEach((b) => b.setAttribute('aria-current', 'page'));
  if (view === 'type' && focus) editor.focus();
  else editor.blur();
  setPeekView(view);
}

// Switch on pointerup rather than waiting for a synthesized click, which
// browsers sometimes drop after complex touch gestures; click stays for
// keyboards and screen readers.
$$('.dock button').forEach((b) => {
  const go = () => {
    if (document.body.dataset.view === b.dataset.view) return;
    haptic(6);
    setView(b.dataset.view);
  };
  b.addEventListener('pointerup', (e) => { if (e.isPrimary) go(); });
  b.addEventListener('click', go);
});

// Hide the dock while the soft keyboard is up, to give typing the room.
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

/* --------------------------------------------------------------- status */

link.on('status', (state, text) => {
  $('#conn').dataset.state = state;
  $('#conn-text').textContent = state === 'connected' ? `On ${text}` : text;
  if (state !== 'connected') $('#conn-rtt').hidden = true;
});

link.on('ping', ({ rtt, flags }) => {
  const el = $('#conn-rtt');
  if (rtt) {
    el.hidden = false;
    el.textContent = `${rtt} ms`;
    el.className = 'conn-rtt' + (rtt > 120 ? ' bad' : rtt > 40 ? ' warn' : '');
  }
  $('#pill-paused').hidden = !(flags & FLAG.PAUSED);
});

/* -------------------------------------------------------------- overlay */

const OVERLAYS = {
  nokey: {
    art: '📷',
    title: 'Point. Scan. Relax.',
    text: 'Open RIFT on your computer and scan its QR code with your camera. That’s the whole setup.',
  },
  unpaired: {
    art: '🔑',
    title: 'New pairing code',
    text: 'Your PC made a fresh code, so this phone needs to scan it again.',
  },
  replaced: {
    art: '🤝',
    title: 'Another phone took over',
    text: 'Someone else is driving your PC right now.',
    action: 'Take back control',
  },
  kicked: {
    art: '👋',
    title: 'Disconnected',
    text: 'Your PC ended this session.',
    action: 'Reconnect',
  },
};

function showOverlay(kind) {
  const o = OVERLAYS[kind];
  $('#overlay-art').textContent = o.art;
  $('#overlay-title').textContent = o.title;
  $('#overlay-text').textContent = o.text;
  const btn = $('#overlay-action');
  btn.hidden = !o.action;
  btn.textContent = o.action || '';
  $('#overlay').hidden = false;
  editor.blur();
}

$('#overlay-action').addEventListener('click', () => {
  $('#overlay').hidden = true;
  resume();
});

link.on('stopped', showOverlay);
link.on('ready', () => {
  $('#overlay').hidden = true;
  if (!storySeen()) showStory(() => setView(prefs.view));
});

/* ----------------------------------------------------------------- boot */

initTyping();
initTrackpad();
initPeek();
initChill();
initStory();
initSettings({
  onForget() {
    forgetPairing();
    showOverlay('nokey');
  },
  onReplay() {
    showStory(() => setView(prefs.view));
  },
});
bindKeys(
  (el) => (el.closest('#keyrow') ? latchedMods() : 0),
  (el) => { if (el.closest('#keyrow') && latchedMods()) clearMods(); },
);

setView(prefs.view, { focus: false });

const pairing = loadPairingKey();
if (!pairing) {
  link.emit('status', 'offline', 'Not paired');
  showOverlay('nokey');
} else {
  useKey(pairing);
  connect();
}

// Exposed for debugging from the console.
window.rift = { link, hasKey, peek: peekState };
