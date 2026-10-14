import { createFrameLoop } from './motion.js';

export function createChartTooltip(tip, container, options = {}) {
  let point = null, content = null, visible = false, destroyed = false;
  const loop = createFrameLoop(() => {
    if (!visible || !point) return false;
    const rect = container.getBoundingClientRect();
    if (point.html !== content) { content = point.html; tip.innerHTML = content; }
    tip.style.transform = `translate3d(calc(${point.x - rect.left}px - 50%), calc(${point.y - rect.top}px - 100% - 14px), 0)`;
    tip.hidden = false;
    return false;
  }, { ...options, minInterval: 1000 / 30 });
  function hide() { visible = false; point = null; tip.hidden = true; loop.setEnabled(false); }
  return {
    show(event, html) {
      if (destroyed) return;
      visible = true;
      point = { x: event.clientX, y: event.clientY, html };
      loop.setEnabled(true);
      loop.request();
    },
    hide,
    destroy() { destroyed = true; hide(); loop.destroy(); },
  };
}
