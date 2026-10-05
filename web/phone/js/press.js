// Buttons that behave like real keys: tap on release, hold to repeat.
import { $$, haptic, host, pc } from './util.js';
import { sendKey } from './link.js';

// Keep the soft keyboard up: a button press must not steal focus.
export function keepFocus(el) {
  el.addEventListener('mousedown', (e) => e.preventDefault());
  el.addEventListener('contextmenu', (e) => e.preventDefault());
}

// The text part of a button: its <b> or <span> if it has one.
const labelOf = (el) => el.querySelector('b, span') || el.lastChild;

/* Tap fires on release, so swiping across a scrollable row of keys doesn't
 * press them (the browser sends pointercancel when it starts scrolling).
 * data-repeat keys auto-repeat while held; data-confirm needs a second tap. */
export function bindPress(el, fire) {
  let timer = 0;
  let live = false;
  let repeated = false;
  const repeat = el.hasAttribute('data-repeat');
  const reset = () => {
    clearTimeout(timer);
    live = false;
    el.classList.remove('pressed');
  };
  keepFocus(el);
  el.addEventListener('pointerdown', () => {
    live = true;
    repeated = false;
    el.classList.add('pressed');
    if (repeat) {
      const loop = (delay) => {
        timer = setTimeout(() => { repeated = true; haptic(4); fire(); loop(45); }, delay);
      };
      loop(380);
    }
  });
  el.addEventListener('pointerup', () => {
    const fireNow = live && !repeated;
    reset();
    if (!fireNow) return;
    if (el.hasAttribute('data-confirm') && !el.classList.contains('armed')) {
      el.classList.add('armed');
      haptic(20);
      const label = labelOf(el);
      el.dataset.label ??= label.textContent;
      label.textContent = 'Tap again';
      setTimeout(() => { el.classList.remove('armed'); label.textContent = el.dataset.label; }, 2200);
      return;
    }
    if (el.classList.contains('armed')) {
      el.classList.remove('armed');
      labelOf(el).textContent = el.dataset.label;
    }
    haptic();
    fire();
  });
  el.addEventListener('pointercancel', reset);
  el.addEventListener('pointerleave', reset);
}

/** Wires every [data-vk] button. getMods supplies latched modifiers. */
export function bindKeys(getMods, afterKey) {
  $$('[data-vk]').forEach((el) => {
    // Read at press time: applyHostOS may remap keys for a Mac.
    bindPress(el, () => {
      sendKey(Number(el.dataset.vk), Number(el.dataset.mods || 0) | getMods(el));
      afterKey(el);
    });
  });
}

let macApplied = false;

/** Swaps labels and shortcuts to their Mac equivalents (data-mac="vk,mods",
 * data-mac-label, data-mac-icon). The server maps Ctrl→⌘, Alt→⌥, Win→⌃. */
export function applyHostOS(mac) {
  if (!mac || macApplied) return;
  macApplied = true;
  host.mac = true;
  document.body.classList.add('host-mac');
  // Static copy: "PC" → "Mac" everywhere it appears as a word.
  const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let n = walk.nextNode(); n; n = walk.nextNode()) {
    if (/\bPC\b/.test(n.nodeValue)) n.nodeValue = pc(n.nodeValue);
  }
  $$('[data-mac]').forEach((el) => {
    const [vk, mods] = el.dataset.mac.split(',');
    el.dataset.vk = vk;
    el.dataset.mods = mods;
  });
  $$('[data-mac-label]').forEach((el) => {
    const last = el.lastChild;
    if (last && last.nodeType === Node.TEXT_NODE) last.textContent = el.dataset.macLabel;
    else el.textContent = el.dataset.macLabel;
    if (el.dataset.label) el.dataset.label = el.dataset.macLabel; // confirm-button restore text
  });
}
