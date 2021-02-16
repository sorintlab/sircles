import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Container, Header, Segment, Form, Button, Grid, Menu, Card, List, Table, Popup, Icon, Dropdown, Label, TextArea, Accordion } from 'semantic-ui-react'
import moment from 'moment'
import marked from 'marked'

import Util from '../modules/Util'
import Avatar from '../components/Avatar'
import CircleSetCoreRoleMember from './CircleSetCoreRoleMember'
import RoleSetMembers from './RoleSetMembers'
import UpdateRoleModal from './UpdateRoleModal'
import CreateRoleModal from './CreateRoleModal'
import DeleteRoleModal from './DeleteRoleModal'
import RoleBreadcrumbs from './RoleBreadcrumbs'

const defaultActiveItem = 'roles'

class Circle extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  resetComponent = () => this.setState(
    {
      activeItem: defaultActiveItem,
      rootRoleUpdateOpen: false,
      roleSetMember: null,
      setCoreRoleMember: null,
      manage: {
        childRoleAdd: false,
        childRoleUpdate: null,
        childRoleDelete: null
      },
      newRoleAdditionalContent: '',
      newRoleAdditionalContentError: null,
      editAdditionalContent: false,
      editAdditionalContentItem: 'write',
      submittingRoleAdditionalContent: false
    })

  handleItemClick = (e, { name }) => this.setState({ activeItem: name })

  handleEditAdditionalContentItemClick = (e, { name }) => this.setState({ editAdditionalContentItem: name })

  componentWillReceiveProps (nextProps) {
    // reset component state when changing route params
    if (this.props.role.uid !== nextProps.role.uid) this.resetComponent()
  }

  setRootRoleUpdateOpen = (open) => {
    this.setState({rootRoleUpdateOpen: open})
  }

  setRoleSetMember = (role) => {
    this.setState({roleSetMember: role})
  }

  setSetCoreRoleMember = (coreRoleUID, roleType) => {
    this.setState({setCoreRoleMember: { coreRoleUID: coreRoleUID, roleType: roleType }})
  }

  unsetSetCoreRoleMember = () => {
    this.setState({setCoreRoleMember: null})
  }

  setRoleSetLeadLink = (circleUID, coreRoleUID) => {
    this.setState({roleSetLeadLink: { circleUID: circleUID, coreRoleUID: coreRoleUID }})
  }

  unsetRoleSetLeadLink = () => {
    this.setState({roleSetLeadLink: null})
  }

  setChildRoleAdd = (open) => {
    this.setState({manage: { childRoleAdd: open }})
  }

  setChildRoleUpdate = (role) => {
    this.setState({manage: { childRoleUpdate: role }})
  }

  setChildRoleDelete = (role) => {
    this.setState({manage: { childRoleDelete: role }})
  }

  setEditRoleAdditionalContent = (edit) => {
    this.setState({newRoleAdditionalContent: this.props.role.additionalContent.content, editAdditionalContent: edit})
  }

  handleRoleAdditionalContentChanged = (e, data) => {
    this.setState({newRoleAdditionalContent: data.value, descriptionError: null})
  }

  handleRoleSetAdditionalContentCancel = (e) => {
    e.preventDefault()
    this.setState({newRoleAdditionalContent: '', editAdditionalContent: false})
  }

  handleRoleSetAdditionalContentSubmit = (e) => {
    e.preventDefault()
    const { role } = this.props
    const { newRoleAdditionalContent } = this.state

    this.setState({submittingRoleAdditionalContent: true})
    this.props.setRoleAdditionalContent(role.uid, newRoleAdditionalContent)
    .then(({ data }) => {
      this.setState({submittingRoleAdditionalContent: false})
      console.log('got data', data)
      if (data.setRoleAdditionalContent.hasErrors) {
        if (data.setRoleAdditionalContent.genericError) {
          this.setState({showError: true, errorMessage: data.setRoleAdditionalContent.genericError})
        }
      } else {
        this.setState({newRoleAdditionalContent: '', editAdditionalContent: false})
      }
    }).catch((error) => {
      this.setState({submittingRoleAdditionalContent: false})
      console.log('there was an error sending the query', error)
    })
  }

  coreRolePriority (r) {
    switch (r.roleType) {
      case 'leadlink': return 1
      case 'replink': return 2
      case 'facilitator': return 3
      case 'secretary': return 4
      default: return 0
    }
  }

  render () {
    const { activeItem, newRoleAdditionalContent, newRoleAdditionalContentError, editAdditionalContent, editAdditionalContentItem, submittingRoleAdditionalContent } = this.state
    const { timeLine, role, roleEventsQuery, viewer } = this.props

    const roleEvents = roleEventsQuery.role.events

    const viewerPermissions = viewer.memberCirclePermissions

    const canEdit = !timeLine

    let tab = null

    if (activeItem === 'members') {
      let coreMembersData = []
      for (let i = 0; i < role.circleMembers.length; i++) {
        const circleMember = role.circleMembers[i]
        if (!circleMember.isCoreMember) continue
        let reasons = []
        if (circleMember.isLeadLink) reasons.push(`lead link`)
        if (circleMember.isDirectMember) reasons.push('directly added as core member')
        if (circleMember.filledRoles.length === 0) reasons.push('fills no role'); else reasons.push(`fills ${circleMember.filledRoles.length} roles`)
        if (circleMember.repLink.length > 0) reasons.push(`replink of ${circleMember.repLink.length} subcircles`)

        const reason = reasons.join(', ')

        coreMembersData.push({
          member: circleMember.member,
          reason: reason
        })
      }

      let otherMembersData = []
      for (let i = 0; i < role.circleMembers.length; i++) {
        const circleMember = role.circleMembers[i]
        if (circleMember.isCoreMember) continue

        otherMembersData.push({
          member: circleMember.member
        })
      }

      tab =
        <div>
          <Header as='h3' block>Core Members</Header>
          <Card.Group>
            {coreMembersData.map(d => (
              <Card key={d.member.uid}>
                <Card.Content style={{'flexGrow': 0}}>
                  <Avatar uid={d.member.uid} size={50} floated='right' inline spaced shape='rounded' />
                  <Card.Header>
                    <Link to={Util.memberUrl(d.member.uid, timeLine)}>
                      {d.member.userName}
                    </Link>
                  </Card.Header>
                </Card.Content>
                <Card.Content extra>
                  <span> ({d.reason})</span>
                </Card.Content>
              </Card>
          ))}
          </Card.Group>

          { otherMembersData.length > 0 &&
            <Header as='h3' block>Other Members</Header>
          }
          { otherMembersData.length > 0 &&
          <Card.Group>
            {otherMembersData.map(d => (
              <Card key={d.member.uid}>
                <Card.Content style={{'flexGrow': 0}}>
                  <Avatar uid={d.member.uid} size={50} floated='right' inline spaced shape='rounded' />
                  <Card.Header>
                    <Link to={Util.memberUrl(d.member.uid, timeLine)}>
                      {d.member.userName}
                    </Link>
                  </Card.Header>
                </Card.Content>
              </Card>
          ))}
          </Card.Group>
          }
        </div>
    }

    if (activeItem === 'roles') {
      let coreRoles = []
      let roles = []
      for (let i = 0; i < role.roles.length; i++) {
        const r = role.roles[i]
        const roleType = r.roleType

        if (roleType === 'normal' || roleType === 'circle') roles.push(r)

        if (roleType === 'leadlink' ||
          roleType === 'replink' ||
          roleType === 'facilitator' ||
          roleType === 'secretary') coreRoles.push(r)
      }

      coreRoles.sort((a, b) => {
        return this.coreRolePriority(a) - this.coreRolePriority(b)
      })

      let coreRolesRows = []
      for (let i = 0; i < coreRoles.length; i++) {
        const r = coreRoles[i]
        const roleType = r.roleType

        let roleMember
        let expireInfo = "doesn't expire"
        let filler = ''
        if (r.roleMembers.length > 0) {
          roleMember = r.roleMembers[0]
        }

        if (roleMember) {
          let member = r.roleMembers[0].member
          const memberLink = Util.memberUrl(member.uid, timeLine)
          filler =
            <Link to={memberLink}>
              <Avatar uid={member.uid} size={30} inline spaced shape='rounded' />
              {member.userName}
            </Link>
          if (moment.utc(roleMember.electionExpiration).isValid()) {
            expireInfo = 'expires on ' + moment.utc(roleMember.electionExpiration).format('L')
          }
        } else {
          filler = 'not filled'
        }

        const roleLink = Util.roleUrl(r.uid, timeLine)

        coreRolesRows.push(
          <Card key={r.uid}>
            <Card.Content style={{'flexGrow': 0}}>
              { roleType === 'leadlink' && canEdit && viewerPermissions.assignRootCircleLeadLink &&
              <Popup className='ui' content='Manage Lead Link' trigger={
                <span className='ui right floated'>
                  <Icon name='user add' link onClick={() => { this.setSetCoreRoleMember(r.uid, roleType) }} />
                </span>
              } />
              }
              { roleType !== 'leadlink' && canEdit && viewerPermissions.assignCircleCoreRoles &&
              <Popup className='ui' content='Manage Core Role' trigger={
                <span className='ui right floated'>
                  <Icon name='user add' link onClick={() => { this.setSetCoreRoleMember(r.uid, roleType) }} />
                </span>
              } />
              }
              <Card.Header>
                <Link to={roleLink}>
                  {r.name}
                </Link>
              </Card.Header>
            </Card.Content>
            <Card.Content>
              <Card.Description>
                {filler}
              </Card.Description>
            </Card.Content>
            { roleType !== 'leadlink' && roleMember &&
              <Card.Content extra>
                {expireInfo}
              </Card.Content>
            }
          </Card>
        )
      }

      roles.sort((a, b) => {
        return a.name.localeCompare(b.name)
      })

      let rolesRows = []
      for (let i = 0; i < roles.length; i++) {
        const r = roles[i]
        const roleType = r.roleType

        let fillers = []
        let extras = []
        if (roleType === 'normal') {
          for (let i = 0, len = r.roleMembers.length; i < len; i++) {
            // Only display max 3 fillers
            if (i >= 3) break

            let member = r.roleMembers[i].member
            let focus = r.roleMembers[i].focus
            let focusString = ''
            if (focus) {
              focusString = ` (${focus})`
            }
            const memberLink = Util.memberUrl(member.uid, timeLine)
            extras.push(
              <List.Item key={member.uid}>
                <Link to={memberLink}>
                  <Avatar uid={member.uid} size={30} inline spaced shape='rounded' />
                  {member.userName}
                </Link>
                {focusString}
              </List.Item>)
          }
          fillers.push(<List>{extras}</List>)
          if (fillers.length === 0) {
            fillers.push(<div key='none'>no members assigned to role</div>)
          }
          if (r.roleMembers.length > 3) { // Other members accordion
            extras = []
            const moreFillersCount = r.roleMembers.length - 3
            for (let i = 3, len = r.roleMembers.length; i < len; i++) {
              let focus = r.roleMembers[i].focus
              let focusString = ''
              if (focus) {
                focusString = ` (${focus})`
              }
              let extramember = r.roleMembers[i].member
              const extramemberLink = Util.memberUrl(extramember.uid, timeLine)
              extras.push(
                <List.Item key={extramember.uid}>
                  <Link to={extramemberLink}>
                    <Avatar uid={extramember.uid} size={30} inline spaced shape='rounded' />
                    {extramember.userName}
                  </Link>
                  {focusString}
                </List.Item>)
            }
            fillers.push(
              <Accordion>
                <Accordion.Title><Icon name='dropdown' />{moreFillersCount} other {moreFillersCount > 1 ? 'members' : 'member' }</Accordion.Title>
                <Accordion.Content>
                  <List>
                    {extras}
                  </List>
                </Accordion.Content>
              </Accordion>
            )
          }
        }

        let leadlink
        if (roleType === 'circle') {
          for (let i = 0, len = r.roles.length; i < len; i++) {
            const sr = r.roles[i]
            if (sr.roleType === 'leadlink') {
              leadlink = sr
              break
            }
          }

          // get leadlink assigned member (there can be only one)
          if (leadlink.roleMembers.length > 0) {
            let leadlinkMember = leadlink.roleMembers[0].member

            const memberLink = Util.memberUrl(leadlinkMember.uid, timeLine)
            fillers.push(
              <List>
                <List.Item key={leadlinkMember.uid}>
                  <Link to={memberLink}>
                    <Avatar uid={leadlinkMember.uid} size={30} inline spaced shape='rounded' />
                    {leadlinkMember.userName}
                  </Link>
                  <span> (Lead Link)</span>
                </List.Item>
              </List>
            )
          } else {
            fillers.push(<div key='none'>no leadlink assigned</div>)
          }
        }

        const roleLink = Util.roleUrl(r.uid, timeLine)

        let cardColor = 'teal'
        if (r.roleType === 'circle') cardColor = 'blue'

        rolesRows.push(
          <Card key={r.uid} color={cardColor}>
            <Card.Content style={{'flexGrow': 0}}>
              {r.roleType === 'normal' && canEdit && viewerPermissions.assignChildRoleMembers &&
              <Popup content='Add/Edit role assignments' trigger={
                <span className='ui right floated'>
                  <Icon name='user add' link onClick={() => { this.setRoleSetMember(r.uid) }} />
                </span>
              } />
          }
              {r.roleType === 'circle' && canEdit && viewerPermissions.assignChildCircleLeadLink &&
              <Popup content='Set circle Lead Link' trigger={
                <span className='ui right floated'>
                  <Icon name='user add' link onClick={() => { this.setRoleSetLeadLink(r.uid, leadlink.uid) }} />
                </span>
              } />
            }
              <Card.Header>
                <Link to={roleLink}>
                  {r.name}
                </Link>
                {r.roleType === 'circle' && <Label className='labelright' color='blue' horizontal basic size='tiny'>Circle</Label> }
              </Card.Header>
            </Card.Content>
            <Card.Content>
              <Card.Description>
                {fillers}
              </Card.Description>
            </Card.Content>
          </Card>
        )
      }

      tab =
        <div>
          <Header as='h3' block>Core Roles</Header>
          <Card.Group>
            {coreRolesRows}
          </Card.Group>
          <Header as='h3' block>Roles</Header>
          <Card.Group>
            {rolesRows}
          </Card.Group>
        </div>
    }

    if (activeItem === 'tensions') {
      tab =
        <div>
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Member</Table.HeaderCell>
                <Table.HeaderCell>Title</Table.HeaderCell>
              </Table.Row>
            </Table.Header>

            <Table.Body>
              {role.tensions.map(t => {
                if (!t.closed) {
                  return (
                    <Table.Row key={t.uid}>
                      <Table.Cell>
                        <Link to={Util.memberUrl(t.member.uid, timeLine)}>
                          <Avatar uid={t.member.uid} size={30} inline spaced shape='rounded' />
                          {t.member.userName}
                        </Link>
                      </Table.Cell>
                      <Table.Cell>
                        <Link to={'/tension/' + t.uid}>
                          {t.title}
                        </Link>
                      </Table.Cell>
                    </Table.Row>
                  )
                }
              })}
            </Table.Body>
          </Table>
        </div>
    }

    if (activeItem === 'manage') {
      let deletableRoles = []
      for (let i = 0; i < role.roles.length; i++) {
        const r = role.roles[i]
        const roleType = r.roleType
        if (roleType !== 'normal' && roleType !== 'circle') continue
        deletableRoles.push(r)
      }

      tab =
        <Segment>
          <Grid stackable columns={3}>
            <Grid.Row>
              <Grid.Column>
                <Button className='icon' onClick={() => { this.setChildRoleAdd(true) }}>Add Child Role</Button>
                { this.state.manage.childRoleAdd ? <CreateRoleModal parentRoleUID={role.uid} open onClose={() => { this.setChildRoleAdd(false) }} /> : null }
              </Grid.Column>
              <Grid.Column>
                <Dropdown text='Edit Child Role' labeled button className='icon'>
                  <Dropdown.Menu>
                    {/* <i class="add user icon"></i> */}
                    <Dropdown.Menu scrolling >
                      {role.roles.map((role) => <Dropdown.Item key={role.uid} text={role.name} onClick={() => { this.setChildRoleUpdate(role) }} />)}
                    </Dropdown.Menu>
                  </Dropdown.Menu>
                </Dropdown>
                { this.state.manage.childRoleUpdate ? <UpdateRoleModal parentRoleUID={role.uid} roleUID={this.state.manage.childRoleUpdate.uid} roleName={this.state.manage.childRoleUpdate.name} open onClose={() => { this.setChildRoleUpdate(null) }} /> : null }
              </Grid.Column>
              <Grid.Column>
                <Dropdown text='Delete Child Role' labeled button className='icon'>
                  <Dropdown.Menu>
                    {/* <i class="add user icon"></i> */}
                    <Dropdown.Menu scrolling >
                      {deletableRoles.map((role) => <Dropdown.Item key={role.uid} text={role.name} onClick={() => { this.setChildRoleDelete(role) }} />)}
                    </Dropdown.Menu>
                  </Dropdown.Menu>
                </Dropdown>
                { this.state.manage.childRoleDelete ? <DeleteRoleModal parentRoleUID={role.uid} roleUID={this.state.manage.childRoleDelete.uid} roleName={this.state.manage.childRoleDelete.name} open onClose={() => { this.setChildRoleUpdate(null) }} /> : null }
              </Grid.Column>
            </Grid.Row>
          </Grid>
        </Segment>
    }

    if (activeItem === 'details') {
      tab =
        <Segment>
          <h3>Purpose</h3>
          <p>{role.purpose}</p>
          <h3>Domains</h3>
          { role.domains.length > 0
          ? <List as='ol'>
            {role.domains.map(domain => (<List.Item as='li' value='-' key={domain.uid}>{ domain.description }</List.Item>))}
          </List>
          : <p>No domains defined</p>
          }
          <h3>Accountabilities</h3>
          { role.accountabilities.length > 0
          ? <List as='ol'>
            { role.accountabilities.map(accountability => (<List.Item as='li' value='-' key={accountability.uid}>{ accountability.description }</List.Item>))}
          </List>
            : <p>No accountabilities defined</p>
          }
          <Header as='h3'>
              Additional Information
          { !editAdditionalContent && canEdit && viewerPermissions.manageRoleAdditionalContent &&
            <Popup trigger={<Icon name='edit' link size='small' onClick={() => { this.setEditRoleAdditionalContent(true) }} />} content='Edit Additional Information' />
          }
          </Header>
          { !role.additionalContent.content && !editAdditionalContent &&
          <span>No additional information</span>
            }
          { role.additionalContent.content && !editAdditionalContent &&
            <div
              className='content'
              dangerouslySetInnerHTML={{
                __html: marked(role.additionalContent.content, {sanitize: true})
              }}
              />
          }
          { editAdditionalContent &&
            <div>
              <Menu attached='top' tabular>
                <Menu.Item name='write' active={editAdditionalContentItem === 'write'} onClick={this.handleEditAdditionalContentItemClick} />
                <Menu.Item name='preview' active={editAdditionalContentItem === 'preview'} onClick={this.handleEditAdditionalContentItemClick} />
              </Menu>
              <Segment clearing attached>
                { editAdditionalContentItem === 'write' &&
                <Form>
                  <TextArea className='edit-content' value={newRoleAdditionalContent} onChange={this.handleRoleAdditionalContentChanged} />
                </Form>
            }
                { editAdditionalContentItem === 'preview' &&
                <div
                  className='content'
                  dangerouslySetInnerHTML={{
                    __html: marked(newRoleAdditionalContent, {sanitize: true})
                  }}
              />
            }
                {newRoleAdditionalContentError && <Label basic color='red' pointing>{newRoleAdditionalContentError}</Label> }
                <Button floated='right' color='green' disabled={submittingRoleAdditionalContent} onClick={this.handleRoleSetAdditionalContentSubmit}>Save</Button>
                <Button floated='right' disabled={submittingRoleAdditionalContent} onClick={this.handleRoleSetAdditionalContentCancel}>Cancel</Button>
              </Segment>
            </div>
          }
        </Segment>
    }

    if (activeItem === 'history') {
      tab =
        <div>
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Time</Table.HeaderCell>
                <Table.HeaderCell>Description</Table.HeaderCell>
                <Table.HeaderCell>Changed Roles</Table.HeaderCell>
              </Table.Row>
            </Table.Header>

            <Table.Body>
              {roleEvents.edges.map(edge => {
                const e = edge.event

                if (e.type !== 'CircleChangesApplied') return

                const ar = e.changedRoles.map(r => {
                  if (r.changeType === 'deleted') return r.previousRole.name
                  return r.role.name
                })
                const ars = ar.join(', ')

                return (
                  <Table.Row key={e.timeLine.id}>
                    <Table.Cell>
                      {moment(e.timeLine.time).format('LLLL')}
                    </Table.Cell>
                    <Table.Cell>
                      <Link to={Util.memberUrl(e.issuer.uid, timeLine)}>
                        <Avatar uid={e.issuer.uid} size={30} inline spaced shape='rounded' />
                        {e.issuer.userName}
                      </Link>
                      {' did some changes'}
                    </Table.Cell>
                    <Table.Cell>
                      {ars}
                    </Table.Cell>
                  </Table.Row>
                )
              })}
            </Table.Body>
          </Table>
          { roleEventsQuery.role.events.hasMoreData &&
            <Button onClick={() => { roleEventsQuery.loadMoreEntries() }}>Load More Users</Button>
        }
        </div>
    }

    return (
      <Container>
        { /* modals */ }
        { this.state.rootRoleUpdateOpen ? <UpdateRoleModal roleUID={role.uid} roleName={role.name} open onClose={() => { this.setRootRoleUpdateOpen(false) }} /> : null }
        { this.state.roleSetMember ? <RoleSetMembers roleUID={this.state.roleSetMember} open onClose={() => { this.setRoleSetMember(null) }} /> : null }
        { this.state.setCoreRoleMember ? <CircleSetCoreRoleMember circleUID={role.uid} coreRoleUID={this.state.setCoreRoleMember.coreRoleUID} roleType={this.state.setCoreRoleMember.roleType} open onClose={() => { this.unsetSetCoreRoleMember() }} /> : null }
        { this.state.roleSetLeadLink ? <CircleSetCoreRoleMember circleUID={this.state.roleSetLeadLink.circleUID} coreRoleUID={this.state.roleSetLeadLink.coreRoleUID} roleType='leadlink' open onClose={() => { this.unsetRoleSetLeadLink() }} /> : null }
        <Segment>
          <RoleBreadcrumbs timeLine={timeLine} role={role} />
        </Segment>
        <Segment>
          <Label as='a' color='blue' ribbon>Circle</Label>
          <h1>{role.name}
            { /* special case for editing root role */ }
            { canEdit && viewerPermissions.manageRootCircle &&
            <Popup trigger={<Icon name='edit' link size='small' onClick={() => { this.setRootRoleUpdateOpen(true) }} />} content='Edit Role' />
            }
          </h1>
          <h3>Purpose</h3>
          { role.purpose !== ''
            ? <p>{role.purpose}</p>
          : <p>Not defined</p>
          }
        </Segment>
        <Menu pointing>
          <Menu.Item name='details' active={activeItem === 'details'} onClick={this.handleItemClick} />
          <Menu.Item name='history' active={activeItem === 'history'} onClick={this.handleItemClick} />
          <Menu.Item name='roles' active={activeItem === 'roles'} onClick={this.handleItemClick} />
          <Menu.Item name='members' active={activeItem === 'members'} onClick={this.handleItemClick} />
          <Menu.Item name='tensions' active={activeItem === 'tensions'} onClick={this.handleItemClick} />
          { canEdit && viewerPermissions.manageChildRoles &&
          <Menu.Item name='manage' active={activeItem === 'manage'} position='right' onClick={this.handleItemClick} />
          }
        </Menu>
        {tab}
      </Container>
    )
  }
}

Circle.propTypes = {
  viewer: PropTypes.object.isRequired,
  role: PropTypes.object.isRequired
}

const setRoleAdditionalContent = gql`
  mutation setRoleAdditionalContent($roleUID: ID!, $content: String!) {
    setRoleAdditionalContent(roleUID: $roleUID, content: $content) {
      hasErrors
      genericError
    }
  }
`

export default compose(
graphql(setRoleAdditionalContent, {
  name: 'setRoleAdditionalContent',
  props: ({ setRoleAdditionalContent }) => ({
    setRoleAdditionalContent: (roleUID, content) => setRoleAdditionalContent({ variables: { roleUID, content }, refetchQueries: ['rolePageQuery'] })
  })
})
)(Circle)
