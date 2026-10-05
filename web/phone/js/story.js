/* First run: the story of the name, told with the interface itself.
 *   0  two places, a sealed line between them
 *   1  the line tears open and light comes through
 *   2  sparks cross from here to there
 */
import { $, haptic, recall, store } from './util.js';
import { tear } from './tear.js';

const story = $('#story');
const next = $('#story-next');
const STEPS = 3;
let step = 0;
let open = 0.6;      // current half-height of the opening (viewBox units)
let target = 0.6;
let raf = 0;
let sparkTimer = 0;
let onDone = null;

function draw() {
  const t = tear(400, 80, open, 3.2, 11);
  $('#story-tear').setAttribute('d', t.lens);
  $('#story-glow').setAttribute('d', tear(400, 80, open * 2.2, 3.2, 11).lens);
}

function animate() {
  open += (target - open) * 0.08;
  draw();
  raf = Math.abs(target - open) > 0.05 ? requestAnimationFrame(animate) : 0;
}

/** A spark rising from "here" into the tear (story step 3). */
function spark() {
  const svg = $('#story-seam').getBoundingClientRect();
  const el = document.createElement('i');
  el.className = 'story-spark';
  const x = svg.left + svg.width * (0.2 + Math.random() * 0.6);
  el.style.left = `${x}px`;
  el.style.top = `${svg.top + svg.height / 2}px`;
  el.style.setProperty('--rise', `${120 + Math.random() * 160}px`);
  story.append(el);
  setTimeout(() => el.remove(), 1100);
}

function go(i) {
  step = i;
  story.dataset.step = String(i);
  next.textContent = i === STEPS - 1 ? 'Open the rift' : 'Next';
  target = i === 0 ? 0.6 : 13;
  if (!raf) raf = requestAnimationFrame(animate);
  clearInterval(sparkTimer);
  if (i === 2) sparkTimer = setInterval(spark, 140);
}

function close() {
  clearInterval(sparkTimer);
  story.hidden = true;
  store('rift.storySeen', true);
  onDone?.();
}

export function showStory(done) {
  onDone = done;
  story.hidden = false;
  open = 0.6;
  draw();
  go(0);
}

export const storySeen = () => !!recall('rift.storySeen');

export function initStory() {
  next.addEventListener('click', () => {
    haptic(step === 0 ? 20 : 8);
    if (step >= STEPS - 1) close();
    else go(step + 1);
  });
  $('#story-skip').addEventListener('click', close);
}
