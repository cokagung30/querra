import { describe, expect, it } from 'vitest'

import en from '@/i18n/en.json'

import { errorText, toAppError } from './apperr'

describe('toAppError', () => {
  it('uses the error Go sent as the cause', () => {
    const cause = { code: 'auth_failed', message: 'password authentication failed', detail: 'app' }
    expect(toAppError(new Error('call failed', { cause }))).toEqual(cause)
  })

  it('turns other errors into internal ones', () => {
    expect(toAppError(new Error('boom'))).toEqual({ code: 'internal', message: 'boom' })
    expect(toAppError('oops')).toEqual({ code: 'internal', message: 'oops' })
  })
})

describe('errorText', () => {
  it('translates the code and its hint', () => {
    expect(errorText({ code: 'auth_failed', message: 'x' })).toEqual({
      message: en.errors.auth_failed,
      hint: en.errorHints.auth_failed,
    })
  })

  it('falls back to the internal message for an unknown code, without a hint', () => {
    expect(errorText({ code: 'from_the_future', message: 'x' })).toEqual({
      message: en.errors.internal,
      hint: undefined,
    })
  })
})
