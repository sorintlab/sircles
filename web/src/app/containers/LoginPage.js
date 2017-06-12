import React from 'react'
import { withApollo } from 'react-apollo'
import { Container } from 'semantic-ui-react'

import config from 'config'
import Auth from '../modules/Auth'
import LoginForm from '../components/LoginForm'

class LoginPage extends React.Component {

  constructor (props, context) {
    super(props, context)

    this.state = {
      error: null,
      disabled: false,
      user: {
        login: '',
        password: ''
      }
    }
  }

  processForm = (event) => {
    // prevent default action. in this case, action is the form submission event
    event.preventDefault()

    // create a string for an HTTP body message
    const login = encodeURIComponent(this.state.user.login)
    const password = encodeURIComponent(this.state.user.password)
    const formData = `login=${login}&password=${password}`

    this.setState({ disabled: true })

    window.fetch(config.apiBaseUrl + '/auth/login', {
      method: 'POST',
      body: formData,
      headers: {
        'Accept': 'application/json',
        'Content-Type': 'application/x-www-form-urlencoded; charset=utf-8'
      }
    })
    .then(response => {
      this.setState({ disabled: false })
      if (response.status !== 200) {
        this.setState({ error: 'Authentication failed' })
        return
      }
      return response.json()
    })
    .then(j => {
      if (!j) return

      this.setState({ error: null })

      // save the token
      Auth.authenticateUser(j.token)
      this.props.client.resetStore()

      // change the current URL to /
      this.props.history.replace('/')
    })
    .catch(error => {
      console.log(error)
      this.setState({ disabled: false, error: 'Authentication failed' })
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
    return (
      <Container>
        <LoginForm
          onSubmit={this.processForm}
          onChange={this.changeUser}
          error={this.state.error}
          successMessage={this.state.successMessage}
          disabled={this.state.disabled}
          user={this.state.user}
      />
      </Container>
    )
  }

}

export default withApollo(LoginPage)
