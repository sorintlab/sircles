import React, { PropTypes } from 'react'
import { Grid, Form, Button, Message } from 'semantic-ui-react'

const LoginForm = ({
  onSubmit,
  onChange,
  error,
  disabled,
  user
}) => (
  <Grid columns={2} centered>
    <Grid.Column>
      <Form action='/' onSubmit={onSubmit}>
        <h2>Login</h2>

        <Form.Input
          placeholder='UserName'
          name='login'
          onChange={onChange}
          value={user.login}
          disabled={disabled}
        />

        <Form.Input
          placeholder='Password'
          type='password'
          name='password'
          onChange={onChange}
          value={user.password}
          disabled={disabled}
        />

        <Button type='submit' disabled={disabled} loading={disabled} primary fluid size='large'>Login</Button>
        <Message negative hidden={!error}>
          <p>{error}</p>
        </Message>

      </Form>
    </Grid.Column>
  </Grid>
)

LoginForm.propTypes = {
  onSubmit: PropTypes.func.isRequired,
  onChange: PropTypes.func.isRequired,
  disabled: PropTypes.object.isRequired,
  user: PropTypes.object.isRequired
}

export default LoginForm
