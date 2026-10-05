// Buttons that behave like real keys: tap on release, hold to repeat.
import { $$, haptic } from './util.js';
import { sendKey } from './link.js';

// Keep the soft keyboard up: a button press must not steal focus.
export function keepFocus(el) {
  el.addEventListener('mousedown', (e) => e.preventDefault());
  el.addEventListener('contextmenu', (e) => e.preventDefault());
}

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
      const label = el.lastChild;
      el.dataset.label ??= label.textContent;
      label.textContent = 'Tap again';
      setTimeout(() => { el.classList.remove('armed'); label.textContent = el.dataset.label; }, 2200);
      return;
    }
    if (el.classList.contains('armed')) {
      el.classList.remove('armed');
      el.lastChild.textContent = el.dataset.label;
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
    const vk = Number(el.dataset.vk);
    const fixed = Number(el.dataset.mods || 0);
    bindPress(el, () => {
      sendKey(vk, fixed | getMods(el));
      afterKey(el);
    });
  });
}
