import { percentile, round } from './stats'

// Frames slower than this are visible stutter on a 60 Hz display.
const LONG_FRAME_MS = 50

export interface PhaseStats {
  phase: string
  durationMs: number
  frames: number
  avgFps: number
  p50FrameMs: number
  p95FrameMs: number
  p99FrameMs: number
  maxFrameMs: number
  longFrames: number
  // Frames in which at least one visible row was still waiting for data.
  blankFrames: number
}

export function summarize(
  phase: string,
  frameMs: number[],
  blankFrames: number,
  durationMs: number,
): PhaseStats {
  return {
    phase,
    durationMs: Math.round(durationMs),
    frames: frameMs.length,
    avgFps: round(durationMs > 0 ? (frameMs.length * 1000) / durationMs : 0),
    p50FrameMs: round(percentile(frameMs, 50)),
    p95FrameMs: round(percentile(frameMs, 95)),
    p99FrameMs: round(percentile(frameMs, 99)),
    maxFrameMs: round(Math.max(0, ...frameMs)),
    longFrames: frameMs.filter((ms) => ms > LONG_FRAME_MS).length,
    blankFrames,
  }
}

interface ScrollPhase {
  phase: string
  axis: 'x' | 'y'
  from: number
  to: number
  durationMs: number
}

// scrollPhase scrolls el linearly from `from` to `to`, one step per animation
// frame, and records how long each frame took.
export function scrollPhase(el: HTMLElement, opts: ScrollPhase): Promise<PhaseStats> {
  const set = (pos: number) => {
    if (opts.axis === 'y') el.scrollTop = pos
    else el.scrollLeft = pos
  }

  return new Promise((resolve) => {
    const frameMs: number[] = []
    let blankFrames = 0
    let start = 0
    let last = 0

    const step = (now: number) => {
      if (start === 0) {
        start = now
      } else {
        frameMs.push(now - last)
        if (el.querySelector('[data-blank]')) blankFrames++
      }
      last = now

      const progress = Math.min(1, (now - start) / opts.durationMs)
      set(opts.from + (opts.to - opts.from) * progress)
      if (progress < 1) requestAnimationFrame(step)
      else resolve(summarize(opts.phase, frameMs, blankFrames, now - start))
    }
    set(opts.from)
    requestAnimationFrame(step)
  })
}

// runGridBench runs the M0.2 scroll phases: ordinary wheel speed, a fast
// fling across all rows, and horizontal scrolling across the columns.
export async function runGridBench(el: HTMLElement): Promise<PhaseStats[]> {
  const maxY = el.scrollHeight - el.clientHeight
  const maxX = el.scrollWidth - el.clientWidth
  const wheelEnd = Math.min(maxY, 2000 * 8)

  const results = [
    await scrollPhase(el, { phase: 'wheel-y', axis: 'y', from: 0, to: wheelEnd, durationMs: 8000 }),
    await scrollPhase(el, {
      phase: 'fling-y',
      axis: 'y',
      from: wheelEnd,
      to: maxY,
      durationMs: 10000,
    }),
    await scrollPhase(el, { phase: 'wheel-x', axis: 'x', from: 0, to: maxX, durationMs: 4000 }),
  ]
  el.scrollTo({ top: 0, left: 0 })
  return results
}
