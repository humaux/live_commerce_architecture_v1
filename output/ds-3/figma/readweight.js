const out = {};
for (const id of ["1401:58002","4:12713","4:12717","1394:103519","1401:38434","4:12734"]) {
  const n = await figma.getNodeByIdAsync(id);
  const o = { name: n.name, fontName: n.fontName, fontWeight: n.fontWeight, lineHeight: n.lineHeight };
  try { o.segments = n.getStyledTextSegments(["fontWeight", "fontName"]).map(s => ({ fontWeight: s.fontWeight, style: s.fontName.style })); } catch (e) { o.segErr = String(e); }
  try { o.variation = n.fontName && n.fontName.variationSettings; } catch (e) {}
  try { o.openType = n.getRangeOpenTypeFeatures ? "n/a" : undefined; } catch (e) {}
  out[id] = o;
}
return JSON.stringify(out);
