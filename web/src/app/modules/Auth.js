class Auth {

  static authenticateUser (token) {
    window.localStorage.setItem('token', token)
  }

  static isUserAuthenticated () {
    return window.localStorage.getItem('token') !== null
  }

  static deauthenticateUser () {
    window.localStorage.removeItem('token')
    // TODO(sgotti) just reload the page so it works also inside the apollo
    // networkInterface afterware that doesn't have access to the react router.
    // Perhaps there are better ways without reloading the page
    window.location.reload()
  }

  static getToken () {
    return window.localStorage.getItem('token')
  }
}

export default Auth
