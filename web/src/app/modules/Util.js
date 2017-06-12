import config from 'config'

class Util {

  static isQueriesError () {
    let error = false
    for (let i = 0; i < arguments.length; i++) {
      const query = arguments[i]
      if (!query) continue
      if (query.error != null) {
        error = true
        break
      }
    }
    return error
  }

  static isQueriesLoading () {
    let loading = false
    for (let i = 0; i < arguments.length; i++) {
      const query = arguments[i]
      if (!query) continue
      if (query.loading) {
        loading = true
        break
      }
    }
    return loading
  }

  static avatarUrl (memberuid, size) {
    let url = `${config.apiBaseUrl}/avatar/${memberuid}`
    if (size) {
      url = url + `?s=${size}`
    }
    return url
  }

  static orgChartUrl (uid, timeLine) {
    if (uid) {
      if (timeLine) return `/timeline/${timeLine}/orgchart/${uid}`
      return `/orgchart/${uid}`
    }
    if (timeLine) return `/timeline/${timeLine}/orgchart`
    return `/orgchart`
  }

  static roleUrl (roleuid, timeLine) {
    if (timeLine) return `/timeline/${timeLine}/role/${roleuid}`
    return `/role/${roleuid}`
  }

  static memberUrl (memberuid, timeLine) {
    if (timeLine) return `/timeline/${timeLine}/member/${memberuid}`
    return `/member/${memberuid}`
  }
}

export default Util
