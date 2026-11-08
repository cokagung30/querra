import 'i18next'

import type en from './en.json'

// Makes t('…') reject keys that are missing from en.json at compile time.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: typeof en }
  }
}
