const ids = ["4:12737","4:12718","1397:105152","1397:105155","1401:38432","1401:38449","1401:38456","1401:58023"];
const hex = (c) => '#' + [c.r, c.g, c.b].map(v => Math.round(v * 255).toString(16).padStart(2, '0')).join('');
const paint = (ps) => (ps === figma.mixed || !ps) ? ps === figma.mixed ? 'mixed' : null : ps.filter(p => p.visible !== false).map(p => p.type === 'SOLID' ? hex(p.color) + (p.opacity != null && p.opacity < 1 ? '@' + p.opacity.toFixed(2) : '') : p.type);
function d(n, depth) {
  const o = { id: n.id, name: n.name, type: n.type, w: Math.round(n.width*100)/100, h: Math.round(n.height*100)/100 };
  if (n.fills !== undefined) o.fill = paint(n.fills);
  if (n.strokes && n.strokes.length) { o.stroke = paint(n.strokes); o.sw = n.strokeWeight === figma.mixed ? [n.strokeTopWeight,n.strokeRightWeight,n.strokeBottomWeight,n.strokeLeftWeight] : n.strokeWeight; o.salign = n.strokeAlign; }
  if (n.cornerRadius !== undefined) o.r = n.cornerRadius === figma.mixed ? [n.topLeftRadius,n.topRightRadius,n.bottomRightRadius,n.bottomLeftRadius] : n.cornerRadius;
  if (n.layoutMode && n.layoutMode !== 'NONE') { o.lay = n.layoutMode; o.pad = [n.paddingTop,n.paddingRight,n.paddingBottom,n.paddingLeft]; o.gap = n.itemSpacing; o.align=[n.primaryAxisAlignItems,n.counterAxisAlignItems]; }
  if (n.type === 'TEXT') { o.chars = n.characters.slice(0,30); o.font = n.fontName === figma.mixed ? 'mixed' : n.fontName.family + ' ' + n.fontName.style; o.size = n.fontSize; o.lh = n.lineHeight; }
  if (n.opacity != null && n.opacity < 1) o.op = n.opacity;
  if (n.visible === false) o.hidden = true;
  if (n.x != null && depth > 0) { o.x = Math.round(n.x*100)/100; o.y = Math.round(n.y*100)/100; }
  if (n.children && depth < 4) o.c = n.children.map(ch => d(ch, depth + 1));
  return o;
}
const out = [];
for (const id of ids) { const n = await figma.getNodeByIdAsync(id); out.push(n ? d(n, 0) : { id, missing: true }); }
return JSON.stringify(out);
