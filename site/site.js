// Draws every seam on the page with the same tear as the app and the logo.
import { tear } from './tear.js';

let id = 0;

function draw(svg) {
  const r = svg.getBoundingClientRect();
  if (!r.width || !r.height) return;
  const vertical = svg.classList.contains('vertical');
  const open = Number(svg.dataset.open || 16);
  const [len, thick] = vertical ? [r.height, r.width] : [r.width, r.height];
  const body = tear(len, thick, open, Math.max(3, open / 3), 5);
  const glow = tear(len, thick, open * 2.2, Math.max(3, open / 3), 5);

  const gid = svg.dataset.gid || (svg.dataset.gid = `g${id++}`);
  svg.setAttribute('viewBox', `0 0 ${r.width} ${r.height}`);
  svg.setAttribute('preserveAspectRatio', 'none');
  const rot = vertical
    ? `rotate(90 ${r.width / 2} ${r.height / 2}) translate(${(r.width - r.height) / 2} ${(r.height - r.width) / 2})`
    : '';
  svg.innerHTML = `
    <defs>
      <linearGradient id="${gid}l" x1="0" x2="1">
        <stop offset="0" stop-color="#ffd25e"/><stop offset="0.5" stop-color="#ff7a3d"/><stop offset="1" stop-color="#ff3d7f"/>
      </linearGradient>
      <filter id="${gid}b" x="-10%" y="-300%" width="120%" height="700%"><feGaussianBlur stdDeviation="${open / 1.6}"/></filter>
    </defs>
    <g transform="${rot}">
      <path d="${glow.lens}" fill="url(#${gid}l)" filter="url(#${gid}b)" opacity="0.8"/>
      <path d="${body.lens}" fill="url(#${gid}l)"/>
      <path d="${body.line}" fill="none" stroke="#fff6e4" stroke-width="1.2" opacity="0.8"/>
    </g>`;
}

const seams = [...document.querySelectorAll('svg.seam')];
const redraw = () => seams.forEach(draw);
addEventListener('resize', redraw);
redraw();
document.fonts?.ready.then(redraw);
