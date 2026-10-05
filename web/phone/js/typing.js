/* Typing.
 *
 * The textarea mirrors what we've typed on the PC since the last reset. On
 * every change we diff old vs new by Unicode code point and send
 * "delete N, insert S". A common-prefix diff handles everything phone
 * keyboards do — autocorrect, swipe, predictions, IME composition, emoji,
 * dictation, even edits in the middle (they are replayed from the end).
 *
 * A zero-width sentinel sits at the start so Backspace on an "empty" field
 * still produces an input event we can forward.
 */
import { $, $$, haptic, prefs } from './util.js';
import { sendEdit, sendKey } from './link.js';
import { keepFocus } from './press.js';

const VK_BACK = 8;
const SENTINEL = '​';

export const editor = $('#editor');
const wrap = editor.parentElement;
let mirror = '';
let composing = false;
let mods = 0;

export const latchedMods = () => mods;

export function clearMods() {
  mods = 0;
  $$('.chip.mod').forEach((c) => c.classList.remove('on'));
}

function resetEditor() {
  mirror = '';
  editor.value = SENTINEL;
  wrap.classList.remove('has-text');
}

export function applySmartTyping() {
  const on = prefs.smart;
  editor.setAttribute('autocorrect', on ? 'on' : 'off');
  editor.setAttribute('autocapitalize', on ? 'sentences' : 'off');
  editor.setAttribute('autocomplete', 'off');
  editor.spellcheck = on;
}

function charVK(ch) {
  const c = ch.toUpperCase();
  if (/^[A-Z0-9]$/.test(c)) return c.charCodeAt(0);
  return { ' ': 32, '\n': 13, '.': 190, ',': 188, '-': 189, '=': 187, '/': 191, ';': 186, "'": 222, '[': 219, ']': 221, '\\': 220, '`': 192 }[ch] || 0;
}

function placeCaretAtEnd() {
  const n = editor.value.length;
  editor.setSelectionRange(n, n);
}

function sync() {
  const v = editor.value;
  const hasSentinel = v.startsWith(SENTINEL);
  const text = hasSentinel ? v.slice(1) : v.replaceAll(SENTINEL, '');

  const a = Array.from(mirror);
  const b = Array.from(text);
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) i++;
  let del = a.length - i;
  const ins = b.slice(i).join('');
  if (!hasSentinel && del === 0 && ins === '') del = 1; // Backspace on empty

  if (mods && !composing && (del === 1 ? ins === '' : del === 0 && b.length - i === 1)) {
    // A latched modifier turns this keystroke into a shortcut (Ctrl+C, Ctrl+Backspace…).
    const vk = del ? VK_BACK : charVK(ins);
    if (vk) {
      sendKey(vk, mods);
      clearMods();
      editor.value = SENTINEL + mirror;
      placeCaretAtEnd();
      return;
    }
  }

  if (del || ins) sendEdit(del, ins);
  mirror = text;
  wrap.classList.toggle('has-text', text !== '' || composing);

  if (!hasSentinel) {
    editor.value = SENTINEL + text;
    placeCaretAtEnd();
  }
  // Start fresh after Enter or when long, so the mirror stays small.
  if (!composing && (ins.includes('\n') || a.length > 4000)) resetEditor();
}

export function initTyping() {
  applySmartTyping();
  resetEditor();
  editor.addEventListener('compositionstart', () => { composing = true; wrap.classList.add('has-text'); });
  editor.addEventListener('compositionend', () => { composing = false; sync(); });
  editor.addEventListener('input', sync);
  editor.addEventListener('focus', () => requestAnimationFrame(() => {
    if (editor.selectionStart === 0) placeCaretAtEnd();
  }));

  $$('[data-mod]').forEach((el) => {
    const bit = Number(el.dataset.mod);
    keepFocus(el);
    el.addEventListener('click', () => {
      haptic();
      mods ^= bit;
      el.classList.toggle('on', !!(mods & bit));
    });
  });
}
