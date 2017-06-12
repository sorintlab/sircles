import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Container, Segment, Message, Button, Header, Label, Form, TextArea } from 'semantic-ui-react'
import marked from 'marked'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class Tension extends React.Component {
  componentWillMount () {
    this.resetComponent()
  }

  componentWillReceiveProps (nextProps) {
    const { tensionQuery } = nextProps

    if (Util.isQueriesError(tensionQuery)) {
      this.props.appError.setError(true)
      return
    }
  }

  resetComponent = () => this.setState({ closeReason: '', showError: false, errorMessage: '' })

  handleErrorMessageDismiss = () => {
    this.setState({ showError: false, errorMessage: '' })
  }

  handleCloseReasonChange = (e, data) => {
    this.setState({ closeReason: data.value })
  }

  handleCloseTension = (e) => {
    e.preventDefault()
    const tensionUID = this.props.match.params.tensionUID
    const { closeReason } = this.state

    let closeTensionChange =
      {
        uid: tensionUID,
        reason: closeReason
      }

    console.log('closeTensionChange', closeTensionChange)

    this.props.closeTension(closeTensionChange)
    .then(({ data }) => {
      console.log('got data', data)
      if (data.closeTension.hasErrors) {
        this.setState({showError: true, errorMessage: data.closeTension.genericError})
      }
      this.props.history.push('/tensions')
    }).catch((error) => {
      console.log('there was an error sending the query', error)
    })
  }

  render () {
    const { tensionQuery } = this.props

    if (Util.isQueriesError(tensionQuery)) {
      return null
    }

    if (Util.isQueriesLoading(tensionQuery)) {
      return null
    }

    const tension = tensionQuery.tension
    const { closeReason, showError, errorMessage } = this.state

    let member = JSON.parse(JSON.stringify(tension.member))

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
          <Segment>
            { !tension.closed &&
            <Button floated='right' color='green' onClick={() => { this.props.history.push(`/tension/${tension.uid}/edit`) }} >Edit</Button>
            }
            <div>
              <Label as='a' color='purple' ribbon>Tension</Label>
              <Header as='h1'>{tension.title}</Header>
              { tension.closed && <Label color='red' horizontal>Closed</Label> }
            </div>
            { tension.role
              ? <h4>Circle {tension.role.name}</h4>
              : <h4>No circle assigned</h4>
          }
          </Segment>
          <Header as='h4' block attached='top'>Description</Header>
          <Segment attached='bottom'>
            <div
              className='content'
              dangerouslySetInnerHTML={{
                __html: marked(tension.description, {sanitize: true})
              }}
              />
          </Segment>
          { tension.closed &&
          <div>
            <Header as='h4' block attached='top'>Closing Reason</Header>
            <Segment clearing attached='bottom'>
              <div
                className='content'
                dangerouslySetInnerHTML={{
                  __html: marked(tension.closeReason, {sanitize: true})
                }}
              />
            </Segment>
          </div>
          }
          { !tension.closed &&
          <div>
            <Header as='h4' block attached='top'>Closing Reason</Header>
            <Segment clearing attached='bottom'>
              { /* TODO(sgotti) enable preview for closing reason since it's rendered as markdown */ }
              <Form onSubmit={this.handleCloseTension}>
                <Form.Field>
                  <TextArea name='closeTension' placeholder='Closing Reason...' value={closeReason} onChange={this.handleCloseReasonChange} />
                </Form.Field>
                <Button floated='right' primary color='red' type='submit' >Close Tension</Button>
              </Form>
            </Segment>
          </div>
          }
          <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
            <Message.Header>Failed to assign Member</Message.Header>
            <p>{errorMessage}</p>
          </Message>
        </div>
      </Container>
    )
  }
}

Tension.propTypes = {
  tensionQuery: PropTypes.object.isRequired
}

const closeTension = gql`
  mutation closeTension($closeTensionChange: CreateTensionChange!) {
    closeTension(closeTensionChange: $closeTensionChange) {
      hasErrors
      genericError
    }
  }
`

const TensionQuery = gql`
  query tensionQuery($tensionUID: ID!) {
    tension(uid: $tensionUID) {
      uid
      title
      description
      role {
        uid
        name
      }
      closed
      closeReason
      member {
        circles {
          role {
            uid
            name
            depth
          }
        }
      }
    }
  }
`

export default compose(
graphql(closeTension, {
  name: 'closeTension',
  props: ({ closeTension }) => ({
    closeTension: (closeTensionChange) => closeTension({ variables: { closeTensionChange }, refetchQueries: [] })
  })
}),
graphql(TensionQuery, {
  name: 'tensionQuery',
  options: props => ({
    variables: {
      tensionUID: props.match.params.tensionUID
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(Tension))
