// afterPaint resolves once the current frame has been painted (two animation
// frames), which is what the startup measurement counts as "ready".
export function afterPaint(): Promise<void> {
  return new Promise((resolve) =>
    requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
  )
}
