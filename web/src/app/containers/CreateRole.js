import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Form, Input, Label, TextArea, Button, Icon, Message, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'

class CreateRole extends React.Component {
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

  roleQueryDone = (roleQuery) => {
    if (roleQuery.error) return

    let parentRole = JSON.parse(JSON.stringify(roleQuery.role))
    this.setState({parentRole: parentRole})
  }

  resetComponent = () => {
    let role = {
      name: '',
      roleType: 'normal',
      purpose: '',
      newDomains: [],
      newAccountabilities: []
    }
    this.setState({role: role, submitting: false, parentRole: null, focus: '', submitFormValid: false, showError: false, errorMessage: '', nameError: null, purposeError: null, newDomainErrors: [], newAccountabilityErrors: []})
  }

  handleNameChange = (e, data) => {
    let {role} = this.state
    role.name = data.value
    this.setState({role: role, nameError: null})
  }

  handlePurposeChange = (e, data) => {
    let {role} = this.state
    role.purpose = data.value
    this.setState({role: role, purposeError: null})
  }

  handleAddNewDomain = (e, data) => {
    e.preventDefault()
    let {role} = this.state
    role.newDomains.push('')
    this.setState({role: role})
  }

  handleDeleteNewDomain = (e, id) => {
    let {role} = this.state
    role.newDomains.splice(id, 1)
    this.setState({role: role, newDomainErrors: []})
  }

  handleNewDomainChange = (e, data) => {
    let {role} = this.state
    role.newDomains[data.name] = data.value
    this.setState({role: role, newDomainErrors: []})
  }

  handleAddNewAccountability = (e, data) => {
    e.preventDefault()
    let {role} = this.state
    role.newAccountabilities.push('')
    this.setState({role: role})
  }

  handleDeleteNewAccountability = (e, id) => {
    let {role} = this.state
    role.newAccountabilities.splice(id, 1)
    this.setState({role: role, newAccountabilityErrors: []})
  }

  handleNewAccountabilityChange = (e, data) => {
    let {role} = this.state
    role.newAccountabilities[data.name] = data.value
    this.setState({role: role, newAccountabilityErrors: []})
  }

  handleCircleChange =(e, data) => {
    let {role} = this.state
    if (data.checked) {
      role.roleType = 'circle'
    } else {
      role.roleType = 'normal'
    }
    this.setState({role: role})
  }

  handleRolesFromParentChange = (i, checked) => {
    let { parentRole } = this.state
    parentRole.roles[i].fromParent = checked
    this.setState({parentRole: parentRole})
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const { role, parentRole } = this.state

    let createRoleChange =
      {
        parentRoleUID: parentRole.uid,
        name: role.name,
        roleType: role.roleType,
        purpose: role.purpose
      }

    let rolesFromParent = []
    for (let i = 0; i < parentRole.roles.length; i++) {
      const r = parentRole.roles[i]
      if (r.fromParent) rolesFromParent.push(r.uid)
    }

    let createDomainChanges = []
    for (let domain of role.newDomains) {
      createDomainChanges.push({ description: domain })
    }

    let createAccountabilityChanges = []
    for (let accountability of role.newAccountabilities) {
      createAccountabilityChanges.push({ description: accountability })
    }

    Object.assign(createRoleChange, { rolesFromParent, createDomainChanges, createAccountabilityChanges })

    console.log('createRoleChange', createRoleChange)

    this.setState({submitting: true})

    this.props.circleCreateChildRole(parentRole.uid, createRoleChange)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.circleCreateChildRole.hasErrors) {
        if (data.circleCreateChildRole.genericError) {
          this.setState({showError: true, errorMessage: data.circleCreateChildRole.genericError})
        }
        if (data.circleCreateChildRole.createRoleChangeErrors.name) {
          this.setState({nameError: data.circleCreateChildRole.createRoleChangeErrors.name})
        }
        if (data.circleCreateChildRole.createRoleChangeErrors.purpose) {
          this.setState({purposeError: data.circleCreateChildRole.createRoleChangeErrors.purpose})
        }

        let newDomainErrors = []
        for (let i = 0; i < data.circleCreateChildRole.createRoleChangeErrors.createDomainChangesErrors.length; i++) {
          const e = data.circleCreateChildRole.createRoleChangeErrors.createDomainChangesErrors[i]
          newDomainErrors[i] = e.description
        }
        this.setState({newDomainErrors: newDomainErrors})

        let newAccountabilityErrors = []
        for (let i = 0; i < data.circleCreateChildRole.createRoleChangeErrors.createAccountabilityChangesErrors.length; i++) {
          const e = data.circleCreateChildRole.createRoleChangeErrors.createAccountabilityChangesErrors[i]
          newAccountabilityErrors[i] = e.description
        }
        this.setState({newAccountabilityErrors: newAccountabilityErrors})
      } else {
        this.props.onClose()
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
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
    const { role, submitting, showError, errorMessage, nameError, purposeError, newDomainErrors, newAccountabilityErrors } = this.state

    let RolesFromParentSelection = () => {
      let { parentRole } = this.state
      let selection = []
      for (let i = 0; i < parentRole.roles.length; i++) {
        const r = parentRole.roles[i]
        if (r.roleType !== 'normal' && r.roleType !== 'circle') continue
        selection.push(
          <Form.Checkbox key={r.uid} label={r.name} name={r.uid} checked={r.fromParent} onChange={(e, data) => { this.handleRolesFromParentChange(i, data.checked) }} />
        )
      }
      return (
        <Form.Field>
          <label>Select which role to move from parent circle</label>
          {selection}
        </Form.Field>
      )
    }

    return (
      <Form onSubmit={this.handleSubmit}>

        <Form.Field>
          <label>Name</label>
          <Input name='name' placeholder='Name' value={role.name} error={nameError} onChange={this.handleNameChange} />
          {nameError && <Label basic color='red' pointing>{nameError}</Label> }
        </Form.Field>

        <Form.Field>
          <label>Purpose</label>
          <Input name='purpose' placeholder='Purpose' value={role.purpose} error={purposeError} onChange={this.handlePurposeChange} />
          {purposeError && <Label basic color='red' pointing>{purposeError}</Label> }
        </Form.Field>
        <label>Domains</label>
        { role.newDomains.map((description, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={newDomainErrors[id] === true}>
                <TextArea name={id} placeholder='Domain' value={description} rows={2} width={15} onChange={this.handleNewDomainChange} />
                {newDomainErrors[id] && <Label basic color='red' pointing>{newDomainErrors[id]}</Label> }
              </Form.Field>
              <Icon size='large' name='delete' link onClick={(e) => { this.handleDeleteNewDomain(e, id) }} />
            </Form.Group>
          </div>
        ))}
        <Form.Field>
          <Button onClick={this.handleAddNewDomain}>Add domain</Button>
        </Form.Field>
        <label>Accountabilities</label>
        { role.newAccountabilities.map((description, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={newAccountabilityErrors[id] === true}>
                <TextArea name={id} placeholder='Accountability' value={description} rows={2} width={15} onChange={this.handleNewAccountabilityChange} />
                {newAccountabilityErrors[id] && <Label basic color='red' pointing>{newAccountabilityErrors[id]}</Label> }
              </Form.Field>
              <Icon size='large' name='delete' link onClick={(e) => { this.handleDeleteNewAccountability(e, id) }} />
            </Form.Group>
          </div>
        ))}
        <Form.Field>
          <Button onClick={this.handleAddNewAccountability}>Add accountability</Button>
        </Form.Field>
        <Form.Checkbox label='Circle' name='circle' checked={role.roleType === 'circle'} onChange={this.handleCircleChange} />
        { role.roleType === 'circle' ? <RolesFromParentSelection /> : null }
        <Form.Field width='two'><Button primary type='submit' disabled={submitting}>Save</Button></Form.Field>
        <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
          <Message.Header>Failed to create Role</Message.Header>
          <p>{errorMessage}</p>
        </Message>
      </Form>
    )
  }
}

CreateRole.propTypes = {
  parentRoleUID: PropTypes.string.isRequired
}

const circleCreateChildRole = gql`
  mutation circleCreateChildRole($roleUID: ID!, $createRoleChange: CreateRoleChange!) {
    circleCreateChildRole(roleUID: $roleUID, createRoleChange: $createRoleChange) {
      hasErrors
      genericError
      createRoleChangeErrors {
        name
        purpose
        createDomainChangesErrors {
          description
        }
        createAccountabilityChangesErrors {
          description
        }
      }
      role {
        uid
        roleType
        name
        purpose
        roles {
          uid
        }
        domains {
          uid
          description
        }
        accountabilities {
          uid
          description
        }
      }
    }
  }
`

const CreateRoleQuery = gql`
  query createRoleQuery($uid: ID!) {
    role(uid: $uid) {
      uid
      name
      roleType
      purpose
      domains {
        uid
        description
      }
      accountabilities {
        uid
        description
      }
      roles {
        uid
        name
        roleType
      }
      parent {
        uid
        name
        roles {
          uid
          name
          roleType
        }
      }
    }
  }
`

export default compose(
graphql(circleCreateChildRole, {
  name: 'circleCreateChildRole',
  props: ({ circleCreateChildRole }) => ({
    circleCreateChildRole: (roleUID, createRoleChange) => circleCreateChildRole({ variables: { roleUID, createRoleChange }, refetchQueries: ['rolePageQuery'] })
  })
}),
graphql(CreateRoleQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.parentRoleUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(CreateRole))
