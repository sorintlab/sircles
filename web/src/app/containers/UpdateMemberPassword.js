import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import { withRouter } from 'react-router-dom'
import gql from 'graphql-tag'
import { Button, Message, Label, Form, Input } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class UpdateMemberPassword extends React.Component {
  componentWillMount () {
    this.resetComponent()
    let formData = {}
    this.setState({ formData: formData })
  }

  componentWillReceiveProps (nextProps) {
    const { memberQuery, viewerQuery } = nextProps

    if (Util.isQueriesError(memberQuery, viewerQuery)) {
      this.props.appError.setError(true)
      return
    }

    if (Util.isQueriesLoading(memberQuery, viewerQuery)) {
      return
    }

    let { memberUID } = this.state
    if (!memberUID) {
      let uid
      if (nextProps.mode === 'self') {
        uid = viewerQuery.viewer.member.uid
      } else {
        uid = memberQuery.member.uid
      }
      this.setState({ memberUID: uid })
    }
  }

  resetComponent = () => this.setState({ memberUID: null, submitting: false, formData: null, showError: false, errorMessage: '', passwordError: null, passwordUpdated: false })

  handleEditCurPassword = (e, data) => {
    const { formData } = this.state
    formData.curPassword = data.value
    this.setState({ formData: formData, passwordError: null })
  }

  handleEditPassword = (e, data) => {
    const { formData } = this.state
    formData.password = data.value
    this.setState({ formData: formData, passwordError: null })
  }

  handleEditRepeatPassword = (e, data) => {
    const { formData } = this.state
    formData.repeatPassword = data.value
    this.setState({ formData: formData, passwordError: null })
  }

  handleUpdatePassword = (e) => {
    e.preventDefault()
    const { memberUID, formData } = this.state

    this.setState({submitting: true})
    this.props.setMemberPassword(memberUID, formData.curPassword, formData.password)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.setMemberPassword.hasErrors) {
        if (data.setMemberPassword.genericError) {
          this.setState({showError: true, errorMessage: data.setMemberPassword.genericError})
        }
      } else {
        this.setState({passwordUpdated: true})
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  render () {
    const { viewerQuery, memberQuery } = this.props

    if (Util.isQueriesError(viewerQuery, memberQuery)) {
      return null
    }

    if (Util.isQueriesLoading(viewerQuery, memberQuery)) {
      return null
    }

    const viewer = viewerQuery.viewer
    const { memberUID, submitting, formData, showError, errorMessage, passwordError, passwordUpdated } = this.state

    console.log('formData', formData)
    console.log('memberUID', memberUID)
    console.log('viewer', viewer)

    let disabled = false

    if (formData.password !== formData.repeatPassword) disabled = true

    if (passwordUpdated) {
      return (
        <Message positive>
          <span>Password successfully updated</span>
        </Message>
      )
    }

    return (
      <div>
        <h2>Change Password</h2>
        <Form className='clearfix'>
          { (!viewer.member.isAdmin || viewer.member.uid === memberUID) &&
            <Form.Field>
              <label>Current Password</label>
              <Input name='password' placeholder='Password' type='password' value={formData.curPassword} onChange={this.handleEditCurPassword} />
              {passwordError && <Label basic color='red' pointing>{passwordError}</Label> }
            </Form.Field>
            }
          <Form.Field>
            <label>New Password</label>
            <Input name='password' placeholder='Password' type='password' value={formData.password} onChange={this.handleEditPassword} />
            {passwordError && <Label basic color='red' pointing>{passwordError}</Label> }
          </Form.Field>
          <Form.Field>
            <label>Repeat New Password</label>
            <Input name='repeat password' placeholder='Repeat Password' type='password' value={formData.repeatPassword} onChange={this.handleEditRepeatPassword} />
          </Form.Field>
          <Button floated='right' color='green' disabled={disabled || submitting} onClick={this.handleUpdatePassword}>Update Password</Button>
        </Form>
        <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
          <p>{errorMessage}</p>
        </Message>
      </div>
    )
  }
}

UpdateMemberPassword.propTypes = {
  mode: PropTypes.string.isRequired
}

const setMemberPassword = gql`
  mutation setMemberPassword($memberUID: ID!, $curPassword: String!, $newPassword: String!) {
    setMemberPassword(memberUID: $memberUID, curPassword: $curPassword, newPassword: $newPassword) {
      hasErrors
      genericError
    }
  }
`

const MemberQuery = gql`
  query memberQuery($uid: ID!) {
    member(uid: $uid) {
      uid
      isAdmin
      userName
      fullName
      email
    }
  }
`

const ViewerQuery = gql`
  query viewerQuery {
    viewer {
      member {
        uid
        isAdmin
        userName
        fullName
      }
    }
  }
`

export default withRouter(compose(
graphql(setMemberPassword, {
  name: 'setMemberPassword',
  props: ({ setMemberPassword }) => ({
    setMemberPassword: (memberUID, curPassword, newPassword) => setMemberPassword({ variables: { memberUID, curPassword, newPassword } })
  })
}),
graphql(MemberQuery, {
  name: 'memberQuery',
  skip: (props) => props.mode === 'self',
  options: props => ({
    variables: {
      uid: props.match.params.memberUID
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(ViewerQuery, {
  name: 'viewerQuery',
  options: () => ({
    fetchPolicy: 'network-only'
  })
}),
)(withError(UpdateMemberPassword)))
