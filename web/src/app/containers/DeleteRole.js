import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Form, Button, Message, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'

class RoleDelete extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  componentWillReceiveProps (nextProps) {
    if (this.props.roleUID !== nextProps.roleUID) this.resetComponent()

    if (nextProps.roleQuery.error) {
      this.props.appError.setError(true)
      return
    }

    if (this.props.roleQuery.loading === true && nextProps.roleQuery.loading === false) this.roleQueryDone(nextProps.roleQuery)
  }

  resetComponent = () => {
    this.setState({role: null, focus: '', submitFormValid: false, showError: false, errorMessage: ''})
  }

  roleQueryDone = (roleQuery) => {
    if (roleQuery.error) return

    let role = JSON.parse(JSON.stringify(roleQuery.role))

    this.setState({role: role})
  }

  handleRolesToParentChange = (i, checked) => {
    let {role} = this.state
    role.roles[i].toParent = checked
    this.setState({role: role})
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const { role } = this.state

    let deleteRoleChange =
      {
        uid: role.uid
      }

    let rolesToParent = []
    for (let i = 0; i < role.roles.length; i++) {
      const r = role.roles[i]
      if (r.toParent) rolesToParent.push(r.uid)
    }

    Object.assign(deleteRoleChange, { rolesToParent })

    console.log('deleteRoleChange', deleteRoleChange)

    this.props.circleDeleteChildRole(this.props.parentRoleUID, deleteRoleChange)
    .then(({ data }) => {
      console.log('got data', data)
      if (data.circleDeleteChildRole.error) {
        this.setState({showError: true, errorMessage: data.circleDeleteChildRole.error})
      }
    }).catch((error) => {
      console.log('there was an error sending the query', error)
    })
    this.props.onClose()
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  render () {
    if (this.props.roleQuery.error != null) {
      return (
        <div />
      )
    }

    if (this.props.roleQuery.loading) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }
    console.log('this.state', this.state)
    const { role, showError, errorMessage } = this.state

    console.log('role', role)

    let ChildRolesToParentSelection = () => {
      let selection = []
      for (let i = 0; i < role.roles.length; i++) {
        const r = role.roles[i]
        console.log('r', r)
        if (r.roleType !== 'normal' && r.roleType !== 'circle') continue
        selection.push(
          <Form.Checkbox key={r.uid} label={r.name} name={r.uid} checked={role.roles[i].toParent} onChange={(e, data) => { this.handleRolesToParentChange(i, data.checked) }} />
        )
      }
      console.log('selection', selection)
      return (
        <Form.Field>
          <label>Select which role to move inside parent circle</label>
          {selection}
        </Form.Field>
      )
    }

    return (
      <Form onSubmit={this.handleSubmit}>
        <ChildRolesToParentSelection />
        <Form.Field width='two'><Button primary type='submit' >Save</Button></Form.Field>
        <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
          <Message.Header>Failed to assign Member</Message.Header>
          <p>{errorMessage}</p>
        </Message>
      </Form>
    )
  }
}

RoleDelete.propTypes = {
  roleQuery: PropTypes.object.isRequired
}

const circleDeleteChildRole = gql`
  mutation circleDeleteChildRole($roleUID: ID!, $deleteRoleChange: DeleteRoleChange!) {
    circleDeleteChildRole(roleUID: $roleUID, deleteRoleChange: $deleteRoleChange) {
      hasErrors
      genericError
    }
  }
`

const RoleDeleteQuery = gql`
  query roleDeleteQuery($uid: ID!) {
    role(uid: $uid) {
      uid
      name
      roleType
      purpose
      roles {
        uid
        name
        roleType
      }
    }
  }
`

export default compose(
graphql(circleDeleteChildRole, {
  name: 'circleDeleteChildRole',
  props: ({ circleDeleteChildRole }) => ({
    circleDeleteChildRole: (roleUID, deleteRoleChange) => circleDeleteChildRole({ variables: { roleUID, deleteRoleChange }, refetchQueries: ['rolePageQuery'] })
  })
}),
graphql(RoleDeleteQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.roleUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(RoleDelete))
