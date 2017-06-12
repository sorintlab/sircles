import React, { PropTypes } from 'react'
import { Link } from 'react-router-dom'
import { Breadcrumb } from 'semantic-ui-react'

import Util from '../modules/Util'

class RoleBreadcrumbs extends React.Component {

  render () {
    const { timeLine, role } = this.props

    let breadcrumbs = []
    for (let i = 0; i < role.parents.length; i++) {
      const parent = role.parents[role.parents.length - 1 - i]
      breadcrumbs.push(
          { key: parent.uid, content: parent.name, as: Link, to: Util.roleUrl(parent.uid, timeLine) }
        )
    }
    breadcrumbs.push(
          { key: role.uid, content: role.name }
        )

    return (
      <Breadcrumb icon='right angle' sections={breadcrumbs} />
    )
  }
}

RoleBreadcrumbs.propTypes = {
  role: PropTypes.object.isRequired
}

export default RoleBreadcrumbs
