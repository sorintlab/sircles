import React from 'react'
import { withApollo } from 'react-apollo'
import { Container, Message } from 'semantic-ui-react'

import config from 'config'

class OIDCLoginPage extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  componentWillReceiveProps (nextProps) {
    if (this.props.location !== nextProps.location) {
      this.resetComponent()
      this.doOIDCAuth()
    }
  }

  resetComponent = () => this.setState({ error: null })

  componentDidMount () {
    this.doOIDCAuth()
  }

  doOIDCAuth () {
    const state = Math.random().toString(36).substr(2, 6)
    window.localStorage.setItem('oidcState', state)

    const formData = `state=${state}`

    // get the oidc auth url from the api server
    window.fetch(config.apiBaseUrl + '/auth/oidcauthurl', {
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
      if (!j) return
      window.location.assign(j.url)
    })
    .catch(error => {
      console.log(error)
      this.setState({ error: 'Authentication failed' })
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

export default withApollo(OIDCLoginPage)
