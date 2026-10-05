// Media and wind down: transport, sleep timer, screen off, lock, sleep.
import { $, $$, haptic, pc } from './util.js';
import { ACTION, FLAG, link, sendAction } from './link.js';
import { bindPress } from './press.js';
import { setLabel } from './seam.js';

let endsAt = 0; // ms timestamp the PC will sleep at, 0 = no timer
let tick = 0;

function fmt(secs) {
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}` : `${m}:${String(s).padStart(2, '0')}`;
}

function render() {
  const left = endsAt ? Math.max(0, Math.round((endsAt - Date.now()) / 1000)) : 0;
  const status = $('#timer-status');
  $('#timer-cancel').hidden = !left;
  setLabel('timer', left ? `sleep in ${fmt(left)}` : null);
  if (left) {
    status.replaceChildren(pc('Your PC sleeps in '), Object.assign(document.createElement('b'), { textContent: fmt(left) }), '. Sleep well.');
  } else {
    status.textContent = pc('Drifting off mid-episode? Your PC can follow you to sleep.');
    $$('.tick').forEach((c) => c.classList.remove('on'));
  }
}

function onPing({ flags, timer }) {
  $('#winddown').hidden = !(flags & FLAG.POWER);
  // Re-sync with the PC's clock; small drift doesn't matter.
  const next = timer ? Date.now() + timer * 1000 : 0;
  if (Math.abs(next - endsAt) > 2000 || !next !== !endsAt) endsAt = next;
  clearInterval(tick);
  if (endsAt) tick = setInterval(render, 1000);
  render();
}

export function initChill() {
  link.on('ping', onPing);

  $$('.tick').forEach((c) => {
    c.addEventListener('click', () => {
      const min = Number(c.dataset.min);
      sendAction(ACTION.SLEEP_TIMER, min);
      haptic(12);
      $$('.tick').forEach((x) => x.classList.toggle('on', x === c));
      endsAt = Date.now() + min * 60000;
      clearInterval(tick);
      tick = setInterval(render, 1000);
      render();
    });
  });
  $('#timer-cancel').addEventListener('click', () => {
    sendAction(ACTION.SLEEP_TIMER, 0);
    endsAt = 0;
    render();
  });

  $$('[data-action]').forEach((el) => {
    bindPress(el, () => sendAction(Number(el.dataset.action)));
  });
}
