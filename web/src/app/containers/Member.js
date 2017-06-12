import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Container, Header, Card, Message, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Util from '../modules/Util'
import Avatar from '../components/Avatar'

class Member extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  resetComponent = () => this.setState({})

  componentWillReceiveProps (nextProps) {
    if (Util.isQueriesError(nextProps.memberQuery)) {
      this.props.appError.setError(true)
      return
    }
  }

  render () {
    const { memberQuery } = this.props

    if (Util.isQueriesError(memberQuery)) {
      return null
    }

    if (Util.isQueriesLoading(memberQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    if (!memberQuery.member) {
      return (
        <Container>
          <Message warning>
            <Message.Header>Member doesn't exist</Message.Header>
          </Message>
        </Container>
      )
    }

    let member = JSON.parse(JSON.stringify(memberQuery.member))

    // sort circle by depth then by name
    member.circles.sort((a, b) => {
      const d = a.role.depth - b.role.depth
      if (d !== 0) return d
      return a.role.name.localeCompare(b.role.name)
    })

    let circleOptions = [
      {
        text: 'No circle',
        value: ''
      }
    ]

    for (let i = 0; i < member.circles.length; i++) {
      const circle = member.circles[i]
      circleOptions.push(
        {
          text: circle.role.name,
          value: circle.role.uid
        }
      )
    }

    return (
      <Container>
        <div>
          <Card>
            <Avatar uid={member.uid} />
            <Card.Content>
              <Card.Header>{member.userName}</Card.Header>
              <Card.Meta>{member.fullName}</Card.Meta>
            </Card.Content>
          </Card>

          <Header as='h3' block>Membership</Header>
          <Card.Group>
            { member.circles.map(circle => (
              <Card key={circle.role.uid}>
                <Card.Content style={{'flexGrow': 0}}>
                  <Card.Header>
                    <Link to={`/role/${circle.role.uid}`}>
                      {circle.role.name}
                    </Link>
                  </Card.Header>
                </Card.Content>
                <Card.Content>
                  {circle.repLink.map(r => (
                    <h4 key={r.uid}>Rep Link of {r.name}</h4>
                ))}
                  { circle.filledRoles.map(r => {
                    if (r.roleType !== 'circle') {
                      return (
                        <h4 key={r.uid}>
                          <Link to={`/role/${r.uid}`}>
                            {r.name}
                          </Link>
                        </h4>
                      )
                    }
                  }
                )}
                </Card.Content>
              </Card>
            ))}
          </Card.Group>
        </div>
      </Container>
    )
  }
}

Member.propTypes = {
  memberQuery: PropTypes.object.isRequired
}

const MemberQuery = gql`
  query memberQuery($timeLineID: TimeLineID, $memberUID: ID!) {
    member(timeLineID: $timeLineID, uid: $memberUID) {
      uid
      isAdmin
      userName
      fullName
      email
      circles {
        role {
          uid
          name
          depth
        }
        filledRoles {
          uid
          name
          depth
        }
        repLink {
          uid
          name
        }
      }
    }
  }
`

export default compose(
graphql(MemberQuery, {
  name: 'memberQuery',
  options: props => ({
    variables: {
      memberUID: props.match.params.memberUID,
      timeLineID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(Member))
