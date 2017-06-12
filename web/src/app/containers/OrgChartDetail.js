import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'

import Util from '../modules/Util'
import { withError } from '../modules/Error'

import Avatar from '../components/Avatar'

class OrgChartDetail extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { orgChartDetailQuery } = nextProps

    if (orgChartDetailQuery.error) {
      this.props.appError.setError(true)
      return
    }
  }

  render () {
    const { timeLine, orgChartDetailQuery } = this.props

    console.log(this.props)

    if (orgChartDetailQuery.error) {
      return null
    }

    if (orgChartDetailQuery.loading) {
      return null
    }

    const role = orgChartDetailQuery.role

    if (!role) {
      return null
    }

    return (
      <div style={{ position: 'static' }}>
        <h1 style={{ textAlign: 'center' }}>{role.name}</h1>
        { role.roleMembers.map(roleMember => (
          <Link style={{ pointerEvents: 'all' }} title={roleMember.member.userName} key={roleMember.member.uid} to={Util.memberUrl(roleMember.member.uid, timeLine)}>
            <Avatar style={{ position: 'static' }} uid={roleMember.member.uid} size={60} inline spaced shape='rounded' />
          </Link>
          ))}
        <h3>Purpose</h3>
        { role.purpose !== '' &&
          <p>{role.purpose}</p> ||
            <p>No purpose defined</p>
}
      </div>
    )
  }
}

OrgChartDetail.propTypes = {
  orgChartDetailQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired,
    role: PropTypes.object
  }).isRequired
}

const OrgChartDetailQuery = gql`
  query orgChartDetailQuery($timeLineID: TimeLineID, $uid: ID!) {
    role(timeLineID: $timeLineID, uid: $uid) {
      uid
      name
      roleType
      purpose
      additionalContent {
        content
      }
      domains {
        uid
        description
      }
      accountabilities {
        uid
        description
      }
      parents {
        uid
        name
      }
      roleMembers {
        member {
          uid
          userName
          fullName
        }
        focus
        electionExpiration
        noCoreMember
      }
    }
  }
`

export default compose(
graphql(OrgChartDetailQuery, {
  name: 'orgChartDetailQuery',
  options: props => ({
    variables: {
      uid: props.roleUID,
      timeLineID: props.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
}),
)(withError(OrgChartDetail))
