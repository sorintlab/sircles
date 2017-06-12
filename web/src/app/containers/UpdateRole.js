import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Form, Input, Label, Button, Icon, Message, TextArea, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'

class UpdateRole extends React.Component {
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
    this.setState({role: null, submitting: false, focus: '', submitFormValid: false, showError: false, errorMessage: '', nameError: null, purposeError: null, domainErrors: [], newDomainErrors: [], accountabilityErrors: [], newAccountabilityErrors: []})
  }

  roleQueryDone = (roleQuery) => {
    if (roleQuery.error) return

    let role = JSON.parse(JSON.stringify(roleQuery.role))
    for (let domain of role.domains) {
      domain.deleted = false
    }
    for (let accountability of role.accountabilities) {
      accountability.deleted = false
    }
    role.newDomains = []
    role.newAccountabilities = []
    role.makeRole = false
    role.makeCircle = false

    this.setState({role: role})
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

  handleDeleteDomain = (e, uid) => {
    let {role} = this.state
    for (let domain of role.domains) {
      if (domain.uid === uid) {
        domain.deleted = !domain.deleted
      }
    }
    this.setState({role: role, domainErrors: []})
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

  handleDomainChange = (e, data) => {
    let {role} = this.state
    role.domains[data.name].description = data.value
    this.setState({role: role, domainErrors: []})
  }

  handleNewDomainChange = (e, data) => {
    let {role} = this.state
    role.newDomains[data.name] = data.value
    this.setState({role: role, newDomainErrors: []})
  }

  handleDeleteAccountability = (e, uid) => {
    let {role} = this.state
    for (let accountability of role.accountabilities) {
      if (accountability.uid === uid) {
        accountability.deleted = !accountability.deleted
      }
    }
    this.setState({role: role, accountabilityErrors: []})
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

  handleAccountabilityChange = (e, data) => {
    let {role} = this.state
    role.accountabilities[data.name].description = data.value
    this.setState({role: role, accountabilityErrors: []})
  }

  handleNewAccountabilityChange = (e, data) => {
    let {role} = this.state
    role.newAccountabilities[data.name] = data.value
    this.setState({role: role, newAccountabilityErrors: []})
  }

  handleMakeRoleChange = (e, data) => {
    let {role} = this.state
    role.makeRole = data.checked
    this.setState({role: role})
  }

  handleMakeCircleChange = (e, data) => {
    let {role} = this.state
    role.makeCircle = data.checked
    this.setState({role: role})
  }

  handleRolesToParentChange = (i, checked) => {
    let {role} = this.state
    role.roles[i].toParent = checked
    this.setState({role: role})
  }

  handleRolesFromParentChange = (i, checked) => {
    let {role} = this.state
    role.parent.roles[i].fromParent = checked
    this.setState({role: role})
  }

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const { role } = this.state
    const curRole = this.props.roleQuery.role

    const isRootRole = !role.parent

    let createDomainChanges = []
    for (let domain of role.newDomains) {
      createDomainChanges.push({ description: domain })
    }

    let updateDomainChanges = []
    for (let i = 0; i < role.domains.length; i++) {
      const domain = role.domains[i]
      const curDomain = curRole.domains[i]
      if (domain.description !== curDomain.description) {
        updateDomainChanges.push({ uid: domain.uid, descriptionChanged: true, description: domain.description })
      }
    }

    let deleteDomainChanges = []
    for (let domain of role.domains) {
      if (domain.deleted) {
        deleteDomainChanges.push({ uid: domain.uid })
      }
    }

    let createAccountabilityChanges = []
    for (let accountability of role.newAccountabilities) {
      createAccountabilityChanges.push({ description: accountability })
    }

    let updateAccountabilityChanges = []
    for (let i = 0; i < role.accountabilities.length; i++) {
      const accountability = role.accountabilities[i]
      const curAccountability = curRole.accountabilities[i]
      if (accountability.description !== curAccountability.description) {
        updateAccountabilityChanges.push({ uid: accountability.uid, descriptionChanged: true, description: accountability.description })
      }
    }

    let deleteAccountabilityChanges = []
    for (let accountability of role.accountabilities) {
      if (accountability.deleted) {
        deleteAccountabilityChanges.push({ uid: accountability.uid })
      }
    }

    let change
    if (isRootRole) {
      change =
      {
        uid: role.uid,
        nameChanged: role.name !== curRole.name,
        name: role.name,
        purposeChanged: role.purpose !== curRole.purpose,
        purpose: role.purpose
      }

      Object.assign(change, { createDomainChanges, updateDomainChanges, deleteDomainChanges, createAccountabilityChanges, updateAccountabilityChanges, deleteAccountabilityChanges })
    } else {
      change =
      {
        uid: role.uid,
        nameChanged: role.name !== curRole.name,
        name: role.name,
        purposeChanged: role.purpose !== curRole.purpose,
        purpose: role.purpose,
        makeRole: role.makeRole,
        makeCircle: role.makeCircle
      }

      let rolesToParent = []
      let rolesFromParent = []
    // root role doesn't have a parent
      if (role.parent) {
        for (let i = 0; i < role.roles.length; i++) {
          const r = role.roles[i]
          if (r.toParent) rolesToParent.push(r.uid)
        }

        for (let i = 0; i < role.parent.roles.length; i++) {
          const r = role.parent.roles[i]
          if (r.fromParent) rolesFromParent.push(r.uid)
        }
      }

      Object.assign(change, { rolesToParent, rolesFromParent, createDomainChanges, updateDomainChanges, deleteDomainChanges, createAccountabilityChanges, updateAccountabilityChanges, deleteAccountabilityChanges })
    }

    console.log('change', change)

    let mutationPromise
    if (isRootRole) {
      mutationPromise = this.props.updateRootRole(change)
    } else {
      mutationPromise = this.props.circleUpdateChildRole(role.parent.uid, change)
    }

    this.setState({submitting: true})

    mutationPromise.then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      let response, changeErrors
      if (isRootRole) {
        response = data.updateRootRole
        changeErrors = response.updateRootRoleChangeErrors
      } else {
        response = data.circleUpdateChildRole
        changeErrors = response.updateRoleChangeErrors
      }
      if (response.hasErrors) {
        if (response.genericError) {
          this.setState({showError: true, errorMessage: response.genericError})
        }
        if (changeErrors.name) {
          this.setState({nameError: changeErrors.name})
        }
        if (changeErrors.purpose) {
          this.setState({purposeError: changeErrors.purpose})
        }

        let domainErrors = []
        for (let i = 0; i < changeErrors.updateDomainChangesErrors.length; i++) {
          const e = changeErrors.updateDomainChangesErrors[i]
          domainErrors[updateDomainChanges[i].uid] = e.description
        }
        this.setState({domainErrors: domainErrors})
        let newDomainErrors = []
        for (let i = 0; i < changeErrors.createDomainChangesErrors.length; i++) {
          const e = changeErrors.createDomainChangesErrors[i]
          newDomainErrors[i] = e.description
        }
        this.setState({newDomainErrors: newDomainErrors})

        let accountabilityErrors = []
        for (let i = 0; i < changeErrors.updateAccountabilityChangesErrors.length; i++) {
          const e = changeErrors.updateAccountabilityChangesErrors[i]
          accountabilityErrors[updateAccountabilityChanges[i].uid] = e.description
        }
        this.setState({accountabilityErrors: accountabilityErrors})
        let newAccountabilityErrors = []
        for (let i = 0; i < changeErrors.createAccountabilityChangesErrors.length; i++) {
          const e = changeErrors.createAccountabilityChangesErrors[i]
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
    const { role, submitting, showError, errorMessage, nameError, purposeError, domainErrors, newDomainErrors, accountabilityErrors, newAccountabilityErrors } = this.state

    const isCoreRole = role.roleType === 'circle' || role.roleType === 'normal'

    let RolesToParentSelection = () => {
      if (!role.parent) return null

      let selection = []
      for (let i = 0; i < role.roles.length; i++) {
        const r = role.roles[i]
        if (r.roleType !== 'normal' && r.roleType !== 'circle') continue
        selection.push(
          <Form.Checkbox key={r.uid} label={r.name} name={r.uid} checked={r.toParent} onChange={(e, data) => { this.handleRolesToParentChange(i, data.checked) }} />
        )
      }
      return (
        <Form.Field>
          <label>Select which role to move to parent circle</label>
          {selection}
        </Form.Field>
      )
    }

    let RolesFromParentSelection = () => {
      if (!role.parent) return null

      let selection = []
      for (let i = 0; i < role.parent.roles.length; i++) {
        const r = role.parent.roles[i]
        if (r.uid === role.uid) continue
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
        { isCoreRole &&
        <Form.Field>
          <label>Name</label>
          <Input name='name' placeholder='Name' value={role.name} error={nameError} onChange={this.handleNameChange} />
          {nameError && <Label basic color='red' pointing>{nameError}</Label> }
        </Form.Field>
      }
        <Form.Field>
          <label>Purpose</label>
          <Input name='purpose' placeholder='Purpose' value={role.purpose} error={purposeError} onChange={this.handlePurposeChange} />
          {purposeError && <Label basic color='red' pointing>{purposeError}</Label> }
        </Form.Field>

        <label>Domains</label>
        { role.domains.map((domain, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={domainErrors[id]}>
                <TextArea name={id} placeholder='Domain' value={domain.description} rows={2} disabled={domain.deleted} onChange={this.handleDomainChange} />
                {domainErrors[domain.uid] && <Label basic color='red' pointing>{domainErrors[domain.uid]}</Label> }
              </Form.Field>
              <Icon size='large' name={domain.deleted ? 'recycle' : 'trash outline'} link onClick={(e) => { this.handleDeleteDomain(e, domain.uid) }} />
            </Form.Group>
          </div>
        ))}
        { role.newDomains.map((description, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={newDomainErrors[id]}>
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
        { role.accountabilities.map((accountability, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={accountabilityErrors[id]}>
                <TextArea name={id} placeholder='Accountability' value={accountability.description} rows={2} disabled={accountability.deleted} onChange={this.handleAccountabilityChange} />
                {accountabilityErrors[accountability.uid] && <Label basic color='red' pointing>{accountabilityErrors[accountability.uid]}</Label> }
              </Form.Field>
              <Icon size='large' name={accountability.deleted ? 'recycle' : 'trash outline'} link onClick={(e) => { this.handleDeleteAccountability(e, accountability.uid) }} />
            </Form.Group>
          </div>
        ))}
        { role.newAccountabilities.map((description, id) => (
          <div key={id}>
            <Form.Group>
              <Form.Field width={15} error={newAccountabilityErrors[id]}>
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
        { role.roleType === 'circle' && role.parent ? <Form.Checkbox label='Transform in a Role' name='makeRole' checked={role.makeRole} onChange={this.handleMakeRoleChange} /> : null }
        { role.roleType === 'normal' ? <Form.Checkbox label='Transform in a Circle' name='makeCircle' checked={role.makeCircle} onChange={this.handleMakeCircleChange} /> : null }
        { !role.makeRole && !role.makeCircle && role.roleType === 'circle' && <RolesToParentSelection /> }
        { !role.makeRole && !role.makeCircle && role.roleType === 'circle' && <RolesFromParentSelection /> }
        { role.makeRole ? <RolesToParentSelection /> : null }
        { role.makeCircle ? <RolesFromParentSelection /> : null }
        <Form.Field width='two'><Button primary type='submit' disabled={submitting} >Save</Button></Form.Field>
        <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
          <Message.Header>Failed to update Role</Message.Header>
          <p>{errorMessage}</p>
        </Message>
      </Form>
    )
  }
}

UpdateRole.propTypes = {
  roleQuery: PropTypes.object.isRequired
}

const updateRootRole = gql`
  mutation updateRootRole($updateRootRoleChange: UpdateRootRoleChange!) {
    updateRootRole(updateRootRoleChange: $updateRootRoleChange) {
      hasErrors
      genericError
      updateRootRoleChangeErrors {
        name
        purpose
        createDomainChangesErrors {
          description
        }
        updateDomainChangesErrors {
          description
        }
        createAccountabilityChangesErrors {
          description
        }
        updateAccountabilityChangesErrors {
          description
        }
      }
      role {
        uid
        roleType
        name
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
        }
      }
    }
  }
`

const circleUpdateChildRole = gql`
  mutation circleUpdateChildRole($roleUID: ID!, $updateRoleChange: UpdateRoleChange!) {
    circleUpdateChildRole(roleUID: $roleUID, updateRoleChange: $updateRoleChange) {
      hasErrors
      genericError
      updateRoleChangeErrors {
        name
        purpose
        createDomainChangesErrors {
          description
        }
        updateDomainChangesErrors {
          description
        }
        createAccountabilityChangesErrors {
          description
        }
        updateAccountabilityChangesErrors {
          description
        }
      }
      role {
        uid
        roleType
        name
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
        }
      }
    }
  }
`

const UpdateRoleQuery = gql`
  query roleUpdateQuery($uid: ID!) {
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
graphql(updateRootRole, {
  name: 'updateRootRole',
  props: ({ updateRootRole }) => ({
    updateRootRole: (updateRootRoleChange) => updateRootRole({ variables: { updateRootRoleChange }, refetchQueries: ['rolePageQuery'] })
  })
}),
graphql(circleUpdateChildRole, {
  name: 'circleUpdateChildRole',
  props: ({ circleUpdateChildRole }) => ({
    circleUpdateChildRole: (roleUID, updateRoleChange) => circleUpdateChildRole({ variables: { roleUID, updateRoleChange }, refetchQueries: ['rolePageQuery'] })
  })
}),
graphql(UpdateRoleQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.roleUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(UpdateRole))
