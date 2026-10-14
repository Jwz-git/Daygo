// One owner, at most one pending frame. A false return puts the loop to sleep.
export function createFrameLoop(render, {
  requestFrame = callback => requestAnimationFrame(callback),
  cancelFrame = id => cancelAnimationFrame(id),
  minInterval = 0,
} = {}) {
  let pending = null, enabled = true, destroyed = false, lastRender = null;
  function request() {
    if (!enabled || destroyed || pending !== null) return;
    pending = requestFrame(time => {
      pending = null;
      if (!enabled || destroyed) return;
      if (lastRender !== null && time - lastRender < minInterval) { request(); return; }
      lastRender = time;
      if (render(time)) request();
    });
  }
  function cancel() { if (pending !== null) cancelFrame(pending); pending = null; }
  return {
    request,
    setEnabled(value) {
      if (destroyed || value === enabled) return;
      enabled = value;
      if (enabled) { lastRender = null; request(); } else cancel();
    },
    destroy() { destroyed = true; cancel(); },
  };
}

export function createScrollActivity(onChange, {
  setTimer = (callback, delay) => setTimeout(callback, delay),
  clearTimer = id => clearTimeout(id), delay = 160,
} = {}) {
  let active = false, pending = null, destroyed = false;
  function setActive(value) { if (value !== active) { active = value; onChange(value); } }
  function reset() { if (pending !== null) clearTimer(pending); pending = null; setActive(false); }
  return {
    update() {
      if (destroyed) return;
      setActive(true);
      if (pending !== null) clearTimer(pending);
      pending = setTimer(() => { pending = null; setActive(false); }, delay);
    },
    reset,
    destroy() { reset(); destroyed = true; },
  };
}

// Pause demo work while it cannot be seen; rebuilding an owner cancels all its work.
export function createPausableTimers({
  now = () => performance.now(),
  setTimer = (callback, delay) => setTimeout(callback, delay),
  clearTimer = id => clearTimeout(id),
} = {}) {
  const tasks = new Set();
  let enabled = true, destroyed = false;
  function arm(task) {
    task.deadline = now() + task.remaining;
    task.id = setTimer(() => {
      tasks.delete(task);
      if (!destroyed) task.callback();
    }, task.remaining);
  }
  return {
    schedule(callback, delay) {
      if (destroyed) return;
      const task = { callback, remaining: delay, id: null, deadline: 0 };
      tasks.add(task);
      if (enabled) arm(task);
    },
    setEnabled(value) {
      if (destroyed || enabled === value) return;
      enabled = value;
      for (const task of tasks) {
        if (enabled) arm(task);
        else { clearTimer(task.id); task.remaining = Math.max(0, task.deadline - now()); task.id = null; }
      }
    },
    destroy() {
      destroyed = true;
      for (const task of tasks) if (task.id !== null) clearTimer(task.id);
      tasks.clear();
    },
  };
}
