export const getAuthRedirect = ({ publicEnabled, to, isLoggedIn }) => {
  if (publicEnabled) return null
  if (to.path === '/login') return null
  if (isLoggedIn) return null

  return { path: '/login', query: { next: to.fullPath } }
}
