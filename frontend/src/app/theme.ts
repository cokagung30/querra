// followSystemTheme applies the OS light/dark appearance by toggling the
// `dark` class that the design tokens in index.css key off. The light / dark /
// system choice in settings comes with M3.5.
export function followSystemTheme(
  root: HTMLElement = document.documentElement,
  media: MediaQueryList = window.matchMedia('(prefers-color-scheme: dark)'),
): () => void {
  const apply = () => root.classList.toggle('dark', media.matches)
  apply()
  media.addEventListener('change', apply)
  return () => media.removeEventListener('change', apply)
}
