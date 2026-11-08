import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import en from './en.json'

// English only for now; the structure is ready for id.json (PRD §7). Keys are
// type-checked through i18next.d.ts.
export const resources = { en: { translation: en } } as const

void i18n.use(initReactI18next).init({
  resources,
  lng: 'en',
  fallbackLng: 'en',
  // Resources are bundled, so initialize synchronously: the first render
  // already has its strings.
  initAsync: false,
  interpolation: {
    escapeValue: false, // React escapes rendered text
  },
})

export default i18n
