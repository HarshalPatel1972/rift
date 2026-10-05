// Copy of web/phone/js/tear.js.
/* The tear: the same ragged, lens-shaped opening as the logo, laid on its
 * side. Both lips follow one fracture line (they were one edge before being
 * pulled apart), pinched shut at the ends and widest in the middle.
 */

function rng(seed) {
  let s = seed;
  return () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
}

/**
 * @param {number} w      width in px
 * @param {number} h      height in px (the tear is centred vertically)
 * @param {number} open   half-height of the opening at its widest, px
 * @param {number} jag    zigzag amplitude, px
 * @returns {{lens: string, line: string}} SVG path data
 */
export function tear(w, h, open, jag = 3, seed = 7) {
  const rand = rng(seed);
  const steps = Math.max(8, Math.round(w / 16));
  const mid = h / 2;
  const top = [], bottom = [], line = [];
  let sign = 1;
  for (let i = 0; i <= steps; i++) {
    const t = i / steps;
    const x = t * w + (i && i < steps ? (rand() - 0.5) * (w / steps) * 0.5 : 0);
    sign = rand() < 0.8 ? -sign : sign;
    const y = mid + (i && i < steps ? jag * (0.35 + rand()) * sign : 0) + Math.sin(t * Math.PI * 2 + 0.3) * jag * 0.6;
    const o = open * Math.pow(Math.sin(Math.PI * t), 1.2);
    top.push([x, y - o]);
    bottom.push([x, y + o * (0.9 + 0.2 * rand())]);
    line.push([x, y]);
  }
  const f = ([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`;
  const lens = `M${f(top[0])} ${top.slice(1).map((p) => `L${f(p)}`).join(' ')} ${bottom.reverse().map((p) => `L${f(p)}`).join(' ')} Z`;
  const path = `M${f(line[0])} ${line.slice(1).map((p) => `L${f(p)}`).join(' ')}`;
  return { lens, line: path };
}
