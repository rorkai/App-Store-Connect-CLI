export const appSectionPrefetchConcurrency = 6;

export async function runWithConcurrency(
  tasks: Array<() => Promise<void>>,
  limit = appSectionPrefetchConcurrency,
) {
  if (tasks.length === 0) return;

  const workerCount = Math.min(tasks.length, Math.max(1, limit));
  let nextIndex = 0;

  async function worker() {
    while (nextIndex < tasks.length) {
      const taskIndex = nextIndex;
      nextIndex += 1;
      await tasks[taskIndex]();
    }
  }

  await Promise.all(Array.from({ length: workerCount }, () => worker()));
}
