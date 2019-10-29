import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Container, Message, Dimmer, Loader } from 'semantic-ui-react'

import Circle from './Circle'
import Role from './Role'

import { withError } from '../modules/Error'

const defaultFetchSize = 25

class RolePage extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { roleQuery, viewerQuery } = nextProps

    if (roleQuery.error ||
      viewerQuery.error) {
      this.props.appError.setError(true)
      return
    }
  }

  render () {
    const { roleQuery, roleEventsQuery, viewerQuery } = this.props
    const timeLine = this.props.match.params.timeLine

    console.log(this.props)

    if (roleQuery.error ||
        roleEventsQuery.error ||
      viewerQuery.error) {
      return null
    }

    if (roleQuery.loading ||
        roleEventsQuery.loading ||
       viewerQuery.loading) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    const role = roleQuery.role
    const viewer = viewerQuery.viewer

    if (!role) {
      return (
        <Container>
          <Message warning>
            <Message.Header>Role doesn't exist</Message.Header>
          </Message>
        </Container>
      )
    }

    if (role.roleType === 'circle') {
      return (
        <Circle timeLine={timeLine} role={role} roleEventsQuery={roleEventsQuery} viewer={viewer} />
      )
    }

    return (
      <Role timeLine={timeLine} role={role} />
    )
  }
}

RolePage.propTypes = {
  roleQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired,
    role: PropTypes.object
  }).isRequired
}

const RolePageQuery = gql`
  query rolePageQuery($timeLineID: TimeLineID, $uid: ID!) {
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
      roles {
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
        circleMembers {
          member {
            uid
            userName
            fullName
          }
          isCoreMember
          isDirectMember
          isLeadLink
          filledRoles {
            uid
          }
          repLink {
            uid
          }
        }
        roleMembers {
          member {
            uid
            userName
            fullName
          }
          focus
          electionExpiration
        }
        roles {
          uid
          name
          roleType
          roleMembers {
            member {
              uid
              userName
              fullName
            }
          }
        }
      }
      circleMembers {
        member {
          uid
          userName
          fullName
          roles {
            role {
              uid
              name
              parent {
                uid
              }
            }
          }
        }
        isCoreMember
        isDirectMember
        isLeadLink
        filledRoles {
          uid
        }
        repLink {
          uid
        }
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
      tensions {
        uid
        title
        description
        closed
        member {
          uid
          userName
          fullName
        }
      }
    }
  }
`
const RoleEventsQuery = gql`
  query roleEventsQuery($timeLineID: TimeLineID, $uid: ID!, $first: Int, $after: String) {
    role(timeLineID: $timeLineID, uid: $uid) {
      events(first: $first, after: $after) {
        hasMoreData
        edges {
          cursor
          event {
            type
            timeLine {
              id
              time
            }
            ... on RoleEventCircleChangesApplied {
              role {
                uid
              }
              issuer {
                uid
                userName
                fullName
              }
              changedRoles {
                changeType
                role {
                  uid
                  name
                }
                previousRole {
                  uid
                  name
                }
                moved {
                  previousParent {
                    uid
                    name
                  }
                  newParent {
                    uid
                    name
                  }
                }
                rolesMovedFromParent {
                  uid
                  name
                }
                rolesMovedToParent {
                  uid
                  name
                }
              }
              rolesFromCircle {
                role {
                  uid
                  name
                }
                previousParent {
                  uid
                  name
                }
                newParent {
                  uid
                  name
                }
              }
              rolesToCircle {
                role {
                  uid
                  name
                }
                previousParent {
                  uid
                  name
                }
                newParent {
                  uid
                  name
                }
              }
            }
          }
        }
      }
    }
  }
`

const ViewerQuery = gql`
  query viewerQuery($roleUID: ID!, $inTimeLine: Boolean!) {
    viewer {
      member {
        uid
        userName
        fullName
        circles {
          role {
            uid
            depth
            name
          }
          isLeadLink
        }
        roles {
          role {
            uid
            depth
            name
            parent {
              uid
              depth
              name
            }
          }
        }
      }
      memberCirclePermissions(roleUID: $roleUID) @skip(if: $inTimeLine) {
        assignChildCircleLeadLink
        assignCircleCoreRoles
        assignChildRoleMembers
        assignCircleDirectMembers
        manageChildRoles
        manageRoleAdditionalContent
        assignRootCircleLeadLink
        manageRootCircle
      }
    }
  }
`

export default compose(
graphql(ViewerQuery, {
  name: 'viewerQuery',
  options: props => ({
    variables: {
      roleUID: props.match.params.roleUID,
      inTimeLine: props.match.params.timeLine != null
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(RolePageQuery, {
  name: 'roleQuery',
  options: props => ({
    variables: {
      uid: props.match.params.roleUID,
      timeLineID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(RoleEventsQuery, {
  options: props => ({
    variables: {
      first: defaultFetchSize,
      uid: props.match.params.roleUID,
      timeLineID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  }),
  props ({ data: { loading, error, refetch, role, fetchMore } }) {
    console.log('loading', loading)
    console.log('error', error)

    let cursor
    const cursors = role && role.events.edges && role.events.edges.map((e) => (e.cursor))
    if (cursors && cursors.length > 0) cursor = cursors[cursors.length - 1]

    return {
      roleEventsQuery: {
        loading,
        error,
        role: role,
        loadMoreEntries: () => {
          return fetchMore({
            variables: {
              first: defaultFetchSize,
              after: cursor
            },
            updateQuery: (previousResult, { fetchMoreResult }) => {
              const newEdges = fetchMoreResult.role.events.edges

              return Object.assign({}, fetchMoreResult, {
                role: Object.assign({}, fetchMoreResult.role, {
                  events: Object.assign({}, fetchMoreResult.role.events, {
                    hasMoreData: fetchMoreResult.role.events.hasMoreData,
                    edges: [...previousResult.role.events.edges, ...newEdges]
                  })
                })
              })
            }
          })
        }
      }
    }
  }
})
)(withError(RolePage))
