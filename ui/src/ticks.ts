// clean axis ticks: about 4 steps of a round number (round in the displayed unit: a power of `base`
// times 1, 2, 2.5, 5 …) reaching above the maximum. base 1024 for bytes (ticks at 1 MB, 2 MB …).
export function cleanTicks(max: number, base: number): number[] {
  if (!(max > 0)) return [0, 1];
  const rough = max / 4;
  let unit = Math.pow(base, Math.max(0, Math.floor(Math.log(rough) / Math.log(base))));
  const stepIn = (u: number) => {
    const inUnit = rough / u;
    const mag = Math.pow(10, Math.floor(Math.log10(inUnit)));
    return [1, 2, 2.5, 5, 10].map((k) => k * mag).find((k) => k >= inUnit) ?? inUnit;
  };
  let k = stepIn(unit);
  if (base !== 10 && k >= 1000) { // "1000 KB" would be the step: say "1 MB" instead
    unit *= base;
    k = stepIn(unit);
  }
  const step = k * unit;
  const out: number[] = [];
  for (let v = 0; v <= max + step * 0.999 && out.length < 7; v += step) out.push(v);
  return out;
}
