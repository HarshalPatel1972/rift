// Settings sheet.
import { $, $$, prefs, savePrefs } from './util.js';
import { deviceName } from './link.js';
import { applySmartTyping, editor } from './typing.js';
import { refreshPeek } from './peek.js';

export function applyLook() {
  document.body.dataset.theme = prefs.theme;
  document.body.classList.toggle('lefty', prefs.lefty);
  const night = prefs.theme === 'night' || (prefs.theme === 'auto' && !matchMedia('(prefers-color-scheme: light)').matches);
  $('meta[name="theme-color"]').content = night ? '#15122b' : '#fff4e4';
}

function bindRange(id, key) {
  const input = $(`#${id}`);
  const out = $(`#${id}-out`);
  const show = () => { out.textContent = `${Number(prefs[key]).toFixed(1)}×`; };
  input.value = prefs[key];
  show();
  input.addEventListener('input', () => {
    prefs[key] = Number(input.value);
    show();
    savePrefs();
  });
}

function bindToggle(id, key, after) {
  const input = $(`#${id}`);
  input.checked = !!prefs[key];
  input.addEventListener('change', () => {
    prefs[key] = input.checked;
    savePrefs();
    after?.();
  });
}

function bindSeg(groupSel, attr, key, after) {
  const buttons = $$(`${groupSel} [${attr}]`);
  const sync = () => buttons.forEach((b) => b.setAttribute('aria-checked', String(b.getAttribute(attr) === prefs[key])));
  buttons.forEach((b) => b.addEventListener('click', () => {
    prefs[key] = b.getAttribute(attr);
    savePrefs();
    sync();
    after?.();
  }));
  sync();
}

export function initSettings({ onForget, onReplay }) {
  const dlg = $('#settings');
  bindSeg('#theme-seg', 'data-theme-opt', 'theme', applyLook);
  bindSeg('#quality-seg', 'data-quality', 'quality', refreshPeek);
  bindRange('sens', 'sens');
  bindRange('scroll', 'scroll');
  bindToggle('typepeek', 'typePeek', refreshPeek);
  bindToggle('natural', 'natural');
  bindToggle('smart', 'smart', applySmartTyping);
  bindToggle('lefty', 'lefty', applyLook);
  bindToggle('haptics', 'haptics');

  const name = $('#device-name');
  name.placeholder = deviceName();
  name.value = prefs.name;
  name.addEventListener('change', () => {
    prefs.name = name.value.trim();
    savePrefs();
  });

  $('#open-settings').addEventListener('click', () => {
    editor.blur();
    dlg.showModal();
  });
  $('#forget').addEventListener('click', () => { dlg.close(); onForget(); });
  $('#replay-story').addEventListener('click', () => { dlg.close(); onReplay(); });
  matchMedia('(prefers-color-scheme: light)').addEventListener('change', applyLook);
  applyLook();
}
