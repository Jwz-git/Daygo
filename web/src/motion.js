// One owner, at most one pending frame. A false return puts the loop to sleep.
export function createFrameLoop(render, {
  requestFrame = callback => requestAnimationFrame(callback),
  cancelFrame = id => cancelAnimationFrame(id),
} = {}) {
  let pending = null, enabled = true, destroyed = false;
  function request() {
    if (!enabled || destroyed || pending !== null) return;
    pending = requestFrame(time => {
      pending = null;
      if (enabled && !destroyed && render(time)) request();
    });
  }
  function cancel() { if (pending !== null) cancelFrame(pending); pending = null; }
  return {
    request,
    setEnabled(value) {
      if (destroyed || value === enabled) return;
      enabled = value;
      if (enabled) request(); else cancel();
    },
    destroy() { destroyed = true; cancel(); },
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
