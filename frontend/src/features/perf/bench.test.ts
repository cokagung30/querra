import { describe, expect, it } from 'vitest'

import { summarize } from './bench'
import { percentile } from './stats'

describe('percentile', () => {
  it('picks the nearest-rank value', () => {
    expect(percentile([5, 1, 4, 2, 3], 50)).toBe(3)
    expect(percentile([5, 1, 4, 2, 3], 100)).toBe(5)
    expect(percentile([], 95)).toBe(0)
  })
})

describe('summarize', () => {
  it('computes frame statistics', () => {
    const frames = [...Array(58).fill(16), 60, 120]
    const stats = summarize('wheel-y', frames, 3, 1000)

    expect(stats.frames).toBe(60)
    expect(stats.avgFps).toBe(60)
    expect(stats.p50FrameMs).toBe(16)
    expect(stats.maxFrameMs).toBe(120)
    expect(stats.longFrames).toBe(2)
    expect(stats.blankFrames).toBe(3)
  })
})
