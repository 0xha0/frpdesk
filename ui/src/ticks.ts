// clean axis ticks: about 4 steps of a round number (round in the displayed unit: a power of `base`
// times 1, 2, 2.5, 5 …) reaching above the maximum. base 1024 for bytes (ticks at 256 KB, 512 KB …
// 1 MB, never "1000 KB"). A step is never below minStep (1: counts and bytes are whole).
export function cleanTicks(max: number, base: number, minStep = 1): number[] {
  if (!(max > 0)) return [0, minStep];
  const rough = Math.max(max / 4, minStep);
  const stepIn = (unit: number) => {
    const inUnit = rough / unit;
    const mag = Math.pow(10, Math.floor(Math.log10(inUnit)));
    return ([1, 2, 2.5, 5, 10].map((k) => k * mag).find((k) => k >= inUnit) ?? inUnit) * unit;
  };
  let unit = Math.pow(base, Math.max(0, Math.floor(Math.log(rough) / Math.log(base))));
  let step = stepIn(unit);
  // the top tick would read "1000 KB": say "1.0 MB" instead, with steps of that unit's fractions
  if (base !== 10 && step * Math.ceil(max / step) >= 1000 * unit) {
    unit *= base;
    step = stepIn(unit);
  }
  step = Math.max(step, minStep);
  if (step >= 1) step = Math.round(step * 1e6) / 1e6;
  const out: number[] = [];
  for (let v = 0; v <= max + step * 0.999 && out.length < 7; v += step) out.push(v);
  return out;
}
