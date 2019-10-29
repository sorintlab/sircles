import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Modal, Container, Form, Button, Table, Icon, Message, Divider } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

import UserSelect from './UserSelect'

class RoleSetMembers extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  componentWillReceiveProps (nextProps) {
    const { roleQuery } = nextProps

    if (Util.isQueriesError(roleQuery)) {
      this.props.appError.setError(true)
      return
    }
  }

  resetComponent = () => {
    this.setState({member: '', focus: '', submitFormValid: false, showError: false, errorMessage: ''})
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const role = this.props.roleQuery.role
    this.props.roleAddMember(role.uid, this.state.member, this.state.focus)
    .then(({ data }) => {
      console.log('got data', data)
      if (data.roleAddMember.hasErrors) {
        this.setState({showError: true, errorMessage: data.roleAddMember.genericError})
      }
    }).catch((error) => {
      console.log('there was an error sending the query', error)
    })
    this.resetComponent()
  }

  removeRoleMember = (e, memberUID) => {
    e.preventDefault()
    const role = this.props.roleQuery.role
    this.setState({ showError: false, errorMessage: '' })
    this.props.roleRemoveMember(role.uid, memberUID)
  }

  handleFocusChange = (e, data) => {
    const value = data.value
    this.setState({ focus: value, showError: false, errorMessage: '' })
  }

  handleMemberChange = (value) => {
    console.log('handleMemberChange', value)
    this.setState({ member: value, submitFormValid: (value.length > 0), showError: false, errorMessage: '' })
  }

  render () {
    const { roleQuery } = this.props
    const { focus, submitFormValid, showError, errorMessage } = this.state

    if (roleQuery.error != null) {
      return null
    }

    if (roleQuery.loading) {
      return null
    }

    const role = roleQuery.role

    return (
      <Modal open={this.props.open} onClose={this.props.onClose} closeIcon='close'>
        <Modal.Header>{role.name} Role Assignments</Modal.Header>
        <Modal.Content>
          <Container>
            <Table basic>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell>Assigned to</Table.HeaderCell>
                  {role.roleType === 'normal' &&
                    <Table.HeaderCell>Focus</Table.HeaderCell>
                  }
                  <Table.HeaderCell />
                </Table.Row>
              </Table.Header>
              <Table.Body>
                { role.roleMembers.map(roleMember => (
                  <Table.Row key={roleMember.member.uid}>
                    <Table.Cell>
                      {roleMember.member.userName}
                    </Table.Cell>
                    {role.roleType === 'normal' &&
                    <Table.Cell>
                      {roleMember.focus}
                    </Table.Cell>
                    }
                    <Table.Cell collapsing>
                      <Icon name='user delete' link onClick={(e) => this.removeRoleMember(e, roleMember.member.uid)} />
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table>
            <Divider />
            <Form onSubmit={this.handleSubmit}>
              <Form.Group widths='equal'>
                <Form.Field control={UserSelect} name='username' onValueChange={this.handleMemberChange} />
                {role.roleType === 'normal' &&
                <Form.Input name='focus' placeholder='Focus' value={focus} onChange={this.handleFocusChange} />
                }
                <Form.Field width='four'><Button primary type='submit' disabled={!submitFormValid}>Add</Button></Form.Field>
              </Form.Group>
            </Form>
            <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
              <Message.Header>Failed to assign Member</Message.Header>
              <p>{errorMessage}</p>
            </Message>
          </Container>
        </Modal.Content>
      </Modal>
    )
  }
}

RoleSetMembers.propTypes = {
  roleQuery: PropTypes.object.isRequired
}

const roleAddMember = gql`
  mutation roleAddMember($roleUID: ID!, $memberUID: ID!, $focus: String) {
    roleAddMember(roleUID: $roleUID, memberUID: $memberUID, focus: $focus) {
      hasErrors
      genericError
    }
  }
`

const roleRemoveMember = gql`
  mutation roleRemoveMember($roleUID: ID!, $memberUID: ID!) {
    roleRemoveMember(roleUID: $roleUID, memberUID: $memberUID) {
      hasErrors
      genericError
    }
  }
`

const RoleSetMemberQuery = gql`
  query roleSetMemberQuery($uid: ID!) {
    role(uid: $uid) {
      uid
      name
      roleType
      roleMembers {
        member {
          uid
          userName
          fullName
        }
        focus
      }
    }
  }
`

export default compose(
graphql(roleAddMember, {
  name: 'roleAddMember',
  props: ({ roleAddMember }) => ({
    roleAddMember: (roleUID, memberUID, focus) => roleAddMember({ variables: { roleUID, memberUID, focus }, refetchQueries: ['roleSetMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(roleRemoveMember, {
  name: 'roleRemoveMember',
  props: ({ roleRemoveMember }) => ({
    roleRemoveMember: (roleUID, memberUID) => roleRemoveMember({ variables: { roleUID, memberUID }, refetchQueries: ['roleSetMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(RoleSetMemberQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.roleUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(RoleSetMembers))
