// Generates the RIFT tear outline used by brand/logo.svg and the UI.
//   node brand/tear.mjs [width] [height]   → prints an SVG path "d"
// The tear is a lens (pinched at both ends, widest in the middle) with
// ragged, irregular lips, so it reads as a torn opening rather than a
// lightning bolt. A fixed seed keeps it identical everywhere.
const W = Number(process.argv[2] || 1024);
const H = Number(process.argv[3] || 1024);

let seed = 7;
const rand = () => ((seed = (seed * 16807) % 2147483647) - 1) / 2147483646;

const top = 0.07 * H, bottom = 0.93 * H, steps = 21;
const left = [], right = [];
let sign = 1;
for (let i = 0; i <= steps; i++) {
  const t = i / steps;
  const y = top + t * (bottom - top) + (i && i < steps ? (rand() - 0.5) * H * 0.02 : 0);
  // One ragged fracture line: a gentle S plus irregular zigzag. Both lips
  // follow it, because they were one edge before being pulled apart.
  sign = rand() < 0.8 ? -sign : sign;
  const jag = (i && i < steps ? W * 0.022 * (0.35 + rand()) : 0) * sign;
  const x = W * (0.5 + 0.04 * Math.sin(t * Math.PI * 2 + 0.3)) + jag;
  // Pulled apart most in the middle, pinched shut at the ends.
  const w = W * 0.088 * Math.pow(Math.sin(Math.PI * t), 1.2) + W * 0.0015;
  left.push([x - w, y]);
  right.push([x + w * (0.9 + 0.2 * rand()), y + W * 0.004]);
}
const fmt = ([x, y]) => `${x.toFixed(1)} ${y.toFixed(1)}`;
const pts = [...left, ...right.reverse()];
console.log(`M${fmt(pts[0])} ` + pts.slice(1).map((p) => `L${fmt(p)}`).join(' ') + ' Z');
