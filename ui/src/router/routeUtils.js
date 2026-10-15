export const buildRootRoute = ({ publicEnabled, HomeView }) => {
  if (publicEnabled) {
    return {
      path: '/',
      name: 'home',
      component: HomeView
    }
  }

  return {
    path: '/',
    redirect: '/modules'
  }
}
