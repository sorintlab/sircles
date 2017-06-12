import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { withRouter } from 'react-router-dom'
import { Container, Segment, Button, Message, Label, Menu, Dropdown, Form, Input, TextArea } from 'semantic-ui-react'
import marked from 'marked'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class EditTension extends React.Component {
  componentWillMount () {
    this.resetComponent()

    if (this.props.type === 'new') {
      let curTension = { description: '', role: { uid: '' } }
      this.setState({ curTension: curTension })
    }
  }

  componentWillReceiveProps (nextProps) {
    const { tensionQuery } = nextProps

    if (!tensionQuery) return
    if (tensionQuery.error) {
      this.props.appError.setError(true)
      return
    }

    if (Util.isQueriesLoading(tensionQuery)) {
      return
    }

    let { curTension } = this.state
    if (!curTension) {
      curTension = this.cloneTension(tensionQuery.tension)
    }
    this.setState({ curTension: curTension })
  }

  resetComponent = () => this.setState({ submitting: false, editItem: 'write', curTension: null, showError: false, errorMessage: '', titleError: null, descriptionError: null })

  cloneTension = (tension) => {
    console.log('cloneTension', tension)
    const curTension = JSON.parse(JSON.stringify(tension))
    if (!curTension.role) {
      curTension.role = { uid: '' }
    }
    console.log('curTension', curTension)
    return curTension
  }

  handleItemClick = (e, { name }) => this.setState({ editItem: name })

  handleCircleChange = (e, data) => {
    const { curTension } = this.state
    curTension.role.uid = data.value
    this.setState({ curTension: curTension })
  }

  handleEditTitle = (e, data) => {
    const { curTension } = this.state
    curTension.title = data.value
    this.setState({ curTension: curTension, titleError: null })
  }

  handleEditDescription = (e, data) => {
    const { curTension } = this.state
    curTension.description = data.value
    this.setState({ curTension: curTension, descriptionError: null })
  }

  handleCancel = (e) => {
    this.close()
  }

  close = () => {
    this.props.history.goBack()
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const { type } = this.props
    const { curTension } = this.state

    if (type === 'edit') {
      let updateTensionChange =
        {
          uid: curTension.uid,
          title: curTension.title,
          description: curTension.description
        }

      if (curTension.role.uid !== '') {
        updateTensionChange.roleUID = curTension.role.uid
      }

      console.log('updateTensionChange', updateTensionChange)

      this.setState({submitting: true})
      this.props.updateTension(updateTensionChange)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.updateTension.hasErrors) {
        if (data.updateTension.genericError) {
          this.setState({showError: true, errorMessage: data.updateTension.genericError})
        }
        if (data.updateTension.updateTensionChangeErrors.title) {
          this.setState({titleError: data.updateTension.updateTensionChangeErrors.title})
        }
        if (data.updateTension.updateTensionChangeErrors.description) {
          this.setState({descriptionError: data.updateTension.updateTensionChangeErrors.description})
        }
      } else {
        this.close()
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
    }

    if (type === 'new') {
      let createTensionChange =
        {
          title: curTension.title,
          description: curTension.description
        }

      if (curTension.role.uid !== '') {
        createTensionChange.roleUID = curTension.role.uid
      }

      console.log('createTensionChange', createTensionChange)

      this.setState({submitting: true})
      this.props.createTension(createTensionChange)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.createTension.hasErrors) {
        if (data.createTension.genericError) {
          this.setState({showError: true, errorMessage: data.createTension.genericError})
        }
        if (data.createTension.createTensionChangeErrors.title) {
          this.setState({titleError: data.createTension.createTensionChangeErrors.title})
        }
        if (data.createTension.createTensionChangeErrors.description) {
          this.setState({descriptionError: data.createTension.createTensionChangeErrors.description})
        }
      } else {
        this.close()
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
    }
  }

  render () {
    const { viewerQuery, tensionQuery } = this.props

    if (Util.isQueriesError(viewerQuery, tensionQuery)) {
      return null
    }

    if (Util.isQueriesLoading(viewerQuery, tensionQuery)) {
      return null
    }

    const viewer = viewerQuery.viewer
    const { type } = this.props
    const { submitting, editItem, curTension, showError, errorMessage, titleError, descriptionError } = this.state

    console.log('curTension', curTension)

    let member
    let title
    if (type === 'edit') {
      member = JSON.parse(JSON.stringify(curTension.member))
      title = 'Edit Tension'
    }
    if (type === 'new') {
      member = JSON.parse(JSON.stringify(viewer.member))
      title = 'New Tension'
    }

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
            <h2>{title}</h2>
          </Segment>
          <Form>
            <Form.Field>
              <label>Title</label>
              <Input name='title' placeholder='Tension Title' value={curTension.title} onChange={this.handleEditTitle} />
              {titleError && <Label basic color='red' pointing>{titleError}</Label> }
            </Form.Field>
            <Form.Field>
              <label>Circle</label>
              <Dropdown selection scrolling options={circleOptions} value={curTension.role.uid} onChange={this.handleCircleChange} />
            </Form.Field>
          </Form>
          <Menu attached='top' tabular>
            <Menu.Item name='write' active={editItem === 'write'} onClick={this.handleItemClick} />
            <Menu.Item name='preview' active={editItem === 'preview'} onClick={this.handleItemClick} />
          </Menu>
          <Segment clearing attached>
            { editItem === 'write' &&
              <Form>
                <TextArea className='edit-content' value={curTension.description} onChange={this.handleEditDescription} />
              </Form>
            }
            { editItem === 'preview' &&
              <div
                className='content'
                dangerouslySetInnerHTML={{
                  __html: marked(curTension.description, {sanitize: true})
                }}
              />
            }
            {descriptionError && <Label basic color='red' pointing>{descriptionError}</Label> }
            <Button floated='right' color='green' disabled={submitting} onClick={this.handleSubmit}>Save</Button>
            <Button floated='right' disabled={submitting} onClick={this.handleCancel}>Cancel</Button>
          </Segment>
          <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
            <Message.Header>Failed to submit Tension</Message.Header>
            <p>{errorMessage}</p>
          </Message>
        </div>
      </Container>
    )
  }
}

EditTension.propTypes = {
  type: PropTypes.string.isRequired
}

const updateTension = gql`
  mutation updateTension($updateTensionChange: CreateTensionChange!) {
    updateTension(updateTensionChange: $updateTensionChange) {
      hasErrors
      genericError
      updateTensionChangeErrors {
        title
        description
      }
      tension {
        uid
        title
        description
      }
    }
  }
`

const createTension = gql`
  mutation createTension($createTensionChange: UpdateTensionChange!) {
    createTension(createTensionChange: $createTensionChange) {
      hasErrors
      genericError
      createTensionChangeErrors {
        title
        description
      }
      tension {
        uid
        title
        description
      }
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

const ViewerQuery = gql`
  query viewerQuery {
    viewer {
      member {
        uid
        userName
        circles {
          role {
            uid
            depth
            name
          }
          isLeadLink
        }
      }
    }
  }
`

export default withRouter(compose(
graphql(createTension, {
  name: 'createTension',
  props: ({ createTension }) => ({
    createTension: (createTensionChange) => createTension({ variables: { createTensionChange }, refetchQueries: ['tensionQuery'] })
  })
}),
graphql(updateTension, {
  name: 'updateTension',
  props: ({ updateTension }) => ({
    updateTension: (updateTensionChange) => updateTension({ variables: { updateTensionChange }, refetchQueries: ['tensionQuery'] })
  })
}),
graphql(TensionQuery, {
  name: 'tensionQuery',
  skip: (props) => props.type === 'new',
  options: props => ({
    variables: {
      tensionUID: props.match.params.tensionUID
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(ViewerQuery, {
  name: 'viewerQuery',
  options: () => ({
    fetchPolicy: 'network-only'
  })
}),
)(withError(EditTension)))
