import React, { PropTypes } from 'react'
import { Container, Segment, Label } from 'semantic-ui-react'

import RoleBreadcrumbs from './RoleBreadcrumbs'

class Role extends React.Component {

  render () {
    const { timeLine, role } = this.props

    console.log(this.props)

    return (
      <Container>
        <Segment>
          <RoleBreadcrumbs timeLine={timeLine} role={role} />
        </Segment>
        <Segment>
          <Label as='a' color='teal' ribbon>Role</Label>
          <h1>{role.name}</h1>
          <h3>Purpose</h3>
          <p>{role.purpose}</p>
          <h3>Domains</h3>
          { role.domains.length > 0
            ? role.domains.map(domain => (<p key={domain.uid}>{ domain.description }</p>))
            : <p>No domains defined</p>
          }
          <h3>Accountabilities</h3>
          { role.accountabilities.length > 0
            ? role.accountabilities.map(accountability => (<p key={accountability.uid}>{ accountability.description }</p>))
            : <p>No accountabilities defined</p>
          }
        </Segment>
      </Container>
    )
  }
}

Role.propTypes = {
  role: PropTypes.object.isRequired
}

export default Role
