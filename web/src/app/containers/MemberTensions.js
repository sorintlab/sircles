import React from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Container, Segment, Message, Button, Table, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'

class MemberTensions extends React.Component {

  componentWillReceiveProps (nextProps) {
    if (nextProps.viewerQuery.error) {
      this.props.appError.setError(true)
      return
    }
  }

  handleNewTension = () => {
    this.props.history.push('/tension/new')
  }

  render () {
    const { viewerQuery } = this.props

    console.log(this.props)

    if (viewerQuery.error) {
      return (
        <Message negative>
          <Message.Header>Error contacting the server</Message.Header>
        </Message>
      )
    }

    if (viewerQuery.loading) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    const viewer = viewerQuery.viewer
    const member = viewer.member

    return (
      <Container>
        <Segment clearing>
          <Button floated='right' color='green' onClick={this.handleNewTension}>New Tension</Button>
          <div>
            <h2>My tensions
          </h2>
          </div>
        </Segment>

        <Table>
          <Table.Header>
            <Table.Row>
              <Table.HeaderCell>Title</Table.HeaderCell>
              <Table.HeaderCell>Circle</Table.HeaderCell>
            </Table.Row>
          </Table.Header>

          <Table.Body>
            {member.tensions.map(t => {
              if (!t.closed) {
                return (
                  <Table.Row key={t.uid}>
                    <Table.Cell>
                      <Link to={'/tension/' + t.uid}>
                        {t.title}
                      </Link>
                    </Table.Cell>
                    <Table.Cell>
                      { t.role
                        ? <Link to={'/role/' + t.role.uid}>
                          {t.role.name}
                        </Link>
                  : ('None')
                  }
                    </Table.Cell>
                  </Table.Row>
                )
              }
            }
          )}
          </Table.Body>
        </Table>
      </Container>
    )
  }
}

MemberTensions.propTypes = {
}

const ViewerQuery = gql`
  query tensionViewerQuery {
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
        tensions {
          uid
          title
          role {
            uid
            name
          }
          closed
        }
      }
    }
  }
`
export default compose(
graphql(ViewerQuery, {
  name: 'viewerQuery',
  options: () => ({
    fetchPolicy: 'network-only'
  })
})
)(withError(MemberTensions))
