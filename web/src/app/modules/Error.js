import React, { PropTypes } from 'react'

class Error {
  constructor () {
    this._error = null
    this._listen = null
  }

  setError (error) {
    this._error = error

    window.setTimeout(this._listen(this._error), 0)
  }

  error () {
    return this._error
  }

  listen (f) {
    this._listen = f
  }
}

export function withError (WrappedComponent) {
  const withDisplayName = `withError(${WrappedComponent.displayName || WrappedComponent.name || 'Component'})`

  class WithError extends React.Component {
    constructor (props, context) {
      super(props, context)
      this.displayName = withDisplayName
      this.WrappedComponent = WrappedComponent

      this.appError = context.appError
    }

    render () {
      const props = Object.assign({}, this.props)
      props.appError = this.appError
      return React.createElement(WrappedComponent, props)
    }
  }

  WithError.contextTypes = {
    appError: PropTypes.object
  }

  return WithError
}

export default Error
