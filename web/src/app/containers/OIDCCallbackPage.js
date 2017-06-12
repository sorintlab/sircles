import React from 'react'
import { withApollo } from 'react-apollo'
import { Container, Message } from 'semantic-ui-react'
import queryString from 'query-string'

import config from 'config'
import Auth from '../modules/Auth'

class OIDCCallbackPage extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  componentWillReceiveProps (nextProps) {
    if (this.props.location !== nextProps.location) {
      this.resetComponent()
      this.doOIDCCallback()
    }
  }

  resetComponent = () => this.setState({ error: null })

  componentDidMount () {
    this.doOIDCCallback()
  }

  doOIDCCallback () {
    console.log('OIDCCallbackPage')

    const params = queryString.parse(window.location.search)

    const code = encodeURIComponent(params.code)
    const state = encodeURIComponent(params.state)

    const localState = window.localStorage.getItem('oidcState')

    if (localState !== state) {
      console.log('received different oidc state than the expected one', state, localState)
    }

    const formData = `code=${code}`

    window.fetch(config.apiBaseUrl + '/auth/login', {
      method: 'POST',
      body: formData,
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/x-www-form-urlencoded; charset=utf-8'
      }
    })
    .then(response => {
      if (response.status !== 200) {
        this.setState({ error: 'Authentication failed' })
        return
      }
      return response.json()
    })
    .then(j => {
      console.log('j', j)

      if (!j) return

      this.setState({ error: null })

      console.log('token', j.token)
      // save the token
      Auth.authenticateUser(j.token)
      this.props.client.resetStore()

      // change the current URL to /
      this.props.history.replace('/')
    })
    .catch(error => {
      console.log(error)
      this.setState({ error: 'Authentication failed' })
    })
  }

  changeUser = (event) => {
    const field = event.target.name
    const user = this.state.user
    user[field] = event.target.value

    this.setState({
      user
    })
  }

  render () {
    const {error} = this.state

    return (
      <Container>
        <Message negative hidden={!error}>
          <p>{error}</p>
        </Message>
      </Container>
    )
  }

}

export default withApollo(OIDCCallbackPage)
