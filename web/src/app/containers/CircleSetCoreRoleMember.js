import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Modal, Container, Form, Button, Table, Icon, Message, Divider } from 'semantic-ui-react'
import moment from 'moment'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

import UserSelect from './UserSelect'

class CircleSetCoreRoleMember extends React.Component {
  constructor () {
    super()
    moment.locale(window.navigator.userLanguage || window.navigator.language)
  }

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
    console.log('resetComponent')
    this.setState({member: '', electionExpiration: '', submitFormValid: false, showError: false, errorMessage: ''})
  }

  isFormValid = (member, electionExpiration) => {
    return member.length > 0 && (electionExpiration === '' || moment.utc(electionExpiration, 'L', true).isValid())
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  handleSubmit = (e) => {
    e.preventDefault()
    let mutation
    if (this.props.roleType === 'leadlink') {
      mutation = this.props.circleSetLeadLinkMember(this.props.circleUID, this.state.member)
      mutation.then(({ data }) => {
        console.log('got data', data)
        if (data.circleSetLeadLinkMember.hasErrors) {
          this.setState({showError: true, errorMessage: data.circleSetLeadLinkMember.genericError})
        }
      }).catch((error) => {
        console.log('there was an error sending the query', error)
      })
    } else {
      let electionExpiration
      if (this.state.electionExpiration !== '') {
        electionExpiration = moment.utc(this.state.electionExpiration).toISOString()
      }
      mutation = this.props.circleSetCoreRoleMember(this.props.roleType, this.props.circleUID, this.state.member, electionExpiration)
      mutation.then(({ data }) => {
        console.log('got data', data)
        if (data.circleSetCoreRoleMember.hasErrors) {
          this.setState({showError: true, errorMessage: data.circleSetCoreRoleMember.genericError})
        }
      }).catch((error) => {
        console.log('there was an error sending the query', error)
      })
    }
    this.resetComponent()
  }

  deleteRoleMember = (e, memberUID) => {
    e.preventDefault()
    this.setState({ showError: false, errorMessage: '' })
    if (this.props.roleType === 'leadlink') {
      this.props.circleUnsetLeadLinkMember(this.props.circleUID, memberUID)
    } else {
      this.props.circleUnsetCoreRoleMember(this.props.roleType, this.props.circleUID, memberUID)
    }
  }

  handleElectionExpirationChange = (e, data) => {
    const value = data.value
    const submitFormValid = this.isFormValid(this.state.member, value)
    this.setState({ electionExpiration: value, submitFormValid: submitFormValid, showError: false, errorMessage: '' })
  }

  handleMemberChange = (value) => {
    console.log('handleMemberChange', value)
    const submitFormValid = this.isFormValid(value, this.state.electionExpiration)
    this.setState({ member: value, submitFormValid: submitFormValid, showError: false, errorMessage: '' })
  }

  render () {
    console.log('render')
    console.log('props', this.props)
    const { roleQuery } = this.props
    const { electionExpiration, submitFormValid, showError, errorMessage } = this.state

    if (roleQuery.error != null) {
      return null
    }

    if (roleQuery.loading) {
      return null
    }

    const role = roleQuery.role

    const dateFormat = moment.localeData().longDateFormat('L')

    return (
      <Modal open={this.props.open} onClose={this.props.onClose} closeIcon='close'>
        <Modal.Header>{role.name} Role Assignment</Modal.Header>
        <Modal.Content>
          <Container>
            <Table basic>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell>Assigned to</Table.HeaderCell>
                  {role.roleType !== 'leadlink' &&
                    <Table.HeaderCell>Expires</Table.HeaderCell>
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
                    {role.roleType !== 'leadlink' &&
                    <Table.Cell>
                      { moment.utc(roleMember.electionExpiration).isValid()
                      ? moment.utc(roleMember.electionExpiration).format('L')
                      : "doesn't expire"
                    }
                    </Table.Cell>
                    }
                    <Table.Cell collapsing>
                      <Icon name='user delete' link onClick={(e) => this.deleteRoleMember(e, roleMember.member.uid)} />
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table>
            <Divider />
            <Form onSubmit={this.handleSubmit}>
              <Form.Group>
                <Form.Input width={6} control={UserSelect} name='username' onValueChange={this.handleMemberChange} />
                {role.roleType !== 'leadlink' &&
                  <Form.Input width={6} name='expires' placeholder={'Elected Until (' + dateFormat + ')'} value={electionExpiration} onChange={this.handleElectionExpirationChange} />
                }
                <Form.Field width={2}><Button primary type='submit' disabled={!submitFormValid}>Add</Button></Form.Field>
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

CircleSetCoreRoleMember.propTypes = {
  roleQuery: PropTypes.object.isRequired,
  roleType: PropTypes.string.isRequired,
  circleUID: PropTypes.string.isRequired
}

const circleSetLeadLinkMember = gql`
  mutation circleSetLeadLinkMember($roleUID: ID!, $memberUID: ID!) {
    circleSetLeadLinkMember(roleUID: $roleUID, memberUID: $memberUID) {
      hasErrors
      genericError
    }
  }
`

const circleUnsetLeadLinkMember = gql`
  mutation circleUnsetLeadLinkMember($roleUID: ID!) {
    circleUnsetLeadLinkMember(roleUID: $roleUID) {
      hasErrors
      genericError
    }
  }
`

const circleSetCoreRoleMember = gql`
  mutation circleSetCoreRoleMember($roleType: RoleType!, $roleUID: ID!, $memberUID: ID!, $electionExpiration: Time) {
    circleSetCoreRoleMember(roleType: $roleType, roleUID: $roleUID, memberUID: $memberUID, electionExpiration: $electionExpiration) {
      hasErrors
      genericError
    }
  }
`

const circleUnsetCoreRoleMember = gql`
  mutation circleUnsetCoreRoleMember($roleType: RoleType!, $roleUID: ID!) {
    circleUnsetCoreRoleMember(roleType: $roleType, roleUID: $roleUID) {
      hasErrors
      genericError
    }
  }
`

const CoreRoleMemberQuery = gql`
  query coreRoleMemberQuery($uid: ID!) {
    role(uid: $uid) {
      uid
      name
      roleType
      roleMembers {
        member {
          uid
          userName
        }
        electionExpiration
      }
    }
  }
`

export default compose(
graphql(circleSetLeadLinkMember, {
  name: 'circleSetLeadLinkMember',
  props: ({ circleSetLeadLinkMember }) => ({
    circleSetLeadLinkMember: (roleUID, memberUID) => circleSetLeadLinkMember({ variables: { roleUID, memberUID }, refetchQueries: ['coreRoleMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(circleUnsetLeadLinkMember, {
  name: 'circleUnsetLeadLinkMember',
  props: ({ circleUnsetLeadLinkMember }) => ({
    circleUnsetLeadLinkMember: (roleUID) => circleUnsetLeadLinkMember({ variables: { roleUID }, refetchQueries: ['coreRoleMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(circleSetCoreRoleMember, {
  name: 'circleSetCoreRoleMember',
  props: ({ circleSetCoreRoleMember }) => ({
    circleSetCoreRoleMember: (roleType, roleUID, memberUID, electionExpiration) => circleSetCoreRoleMember({ variables: { roleType, roleUID, memberUID, electionExpiration }, refetchQueries: ['coreRoleMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(circleUnsetCoreRoleMember, {
  name: 'circleUnsetCoreRoleMember',
  props: ({ circleUnsetCoreRoleMember }) => ({
    circleUnsetCoreRoleMember: (roleType, roleUID) => circleUnsetCoreRoleMember({ variables: { roleType, roleUID }, refetchQueries: ['coreRoleMemberQuery', 'rolePageQuery', 'memberQuery'] })
  })
}),
graphql(CoreRoleMemberQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.coreRoleUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(CircleSetCoreRoleMember))
