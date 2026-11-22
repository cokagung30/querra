import i18n, { type ParseKeys } from 'i18next'

// AppError is the shape of internal/apperr.Error, which the backend sends as
// the rejected call's error.cause (PLAN §3.9).
export interface AppError {
  code: string
  message: string
  detail?: string
  hint?: string
  position?: number
}

export function toAppError(err: unknown): AppError {
  if (err instanceof Error) {
    const cause = err.cause as Partial<AppError> | undefined
    if (cause && typeof cause.code === 'string' && cause.code !== '') return cause as AppError
    return { code: 'internal', message: err.message }
  }
  return { code: 'internal', message: String(err) }
}

// errorText is what the user reads for an error: the translated text for its
// code (errors.<code>) and, when there is one, a hint (errorHints.<code>).
export function errorText(err: AppError): { message: string; hint?: string } {
  const key = `errors.${err.code}`
  const hintKey = `errorHints.${err.code}`
  return {
    message: i18n.t((i18n.exists(key) ? key : 'errors.internal') as ParseKeys),
    hint: i18n.exists(hintKey) ? i18n.t(hintKey as ParseKeys) : undefined,
  }
}
