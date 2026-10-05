// The first-run story: four swipeable cards.
import { $, $$, haptic, recall, store } from './util.js';

const story = $('#story');
const track = $('#story-track');
const dots = $$('#story-dots i');
const next = $('#story-next');
let onDone = null;

function index() {
  return Math.round(track.scrollLeft / track.clientWidth);
}

function sync() {
  const i = index();
  dots.forEach((d, j) => d.classList.toggle('on', j === i));
  next.textContent = i === dots.length - 1 ? "Let's go" : 'Next';
}

function close() {
  story.hidden = true;
  store('rift.storySeen', true);
  onDone?.();
}

export function showStory(done) {
  onDone = done;
  story.hidden = false;
  track.scrollLeft = 0;
  sync();
}

export const storySeen = () => !!recall('rift.storySeen');

export function initStory() {
  track.addEventListener('scroll', () => requestAnimationFrame(sync), { passive: true });
  next.addEventListener('click', () => {
    haptic();
    const i = index();
    if (i >= dots.length - 1) close();
    else track.scrollTo({ left: (i + 1) * track.clientWidth, behavior: 'smooth' });
  });
  $('#story-skip').addEventListener('click', close);
}
