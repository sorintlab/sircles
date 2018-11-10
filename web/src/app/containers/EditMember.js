import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { withRouter } from 'react-router-dom'
import { Button, Image, Message, Label, Form, Input, Checkbox, Modal, Dimmer, Loader } from 'semantic-ui-react'
import ReactCrop from 'react-image-crop'

import { withError } from '../modules/Error'
import Util from '../modules/Util'
import Avatar from '../components/Avatar'

class EditMember extends React.Component {
  componentWillMount () {
    this.resetComponent()

    if (this.props.type === 'new') {
      let curMember = {}
      curMember = { uid: '' }
      this.setState({ curMember: curMember })
    }
  }

  componentWillReceiveProps (nextProps) {
    const { memberQuery, viewerQuery } = nextProps

    let { curMember } = this.state

    if (this.props.mode !== nextProps.mode || this.props.type !== nextProps.type) {
      this.resetComponent()
      curMember = null
    }

    if (this.props.type === 'new') {
      let curMember = {}
      curMember = { uid: '' }
      this.setState({ curMember: curMember })
    }

    if (Util.isQueriesError(memberQuery, viewerQuery)) {
      this.props.appError.setError(true)
      return
    }

    if (Util.isQueriesLoading(memberQuery, viewerQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    if (!curMember) {
      let m
      if (nextProps.mode === 'self') {
        m = this.cloneMember(viewerQuery.viewer.member)
      } else if (nextProps.type === 'edit') {
        m = this.cloneMember(memberQuery.member)
      }
      this.setState({ curMember: m })
    }
  }

  resetComponent = () => {
    this.setState({ submitting: false,
      curMember: null,
      avatarUpdated: false,
      cropData: null,
      crop: { aspect: 1 },
      showError: false,
      errorMessage: '',
      userNameError: null,
      fullNameError: null,
      emailError: null,
      passwordError: null,
      avatarError: null,
      profileCreated: false,
      profileUpdated: false })
  }

  cloneMember = (member) => {
    let curMember = JSON.parse(JSON.stringify(member))
    if (!curMember) {
      curMember = { uid: '' }
    }
    return curMember
  }

  handleEditUserName = (e, data) => {
    const { curMember } = this.state
    curMember.userName = data.value
    this.setState({ curMember: curMember, userNameError: null })
  }

  handleEditFullName = (e, data) => {
    const { curMember } = this.state
    curMember.fullName = data.value
    this.setState({ curMember: curMember, fullNameError: null })
  }

  handleEditEmail = (e, data) => {
    const { curMember } = this.state
    curMember.email = data.value
    this.setState({ curMember: curMember, emailError: null })
  }

  handleEditCurPassword = (e, data) => {
    const { curMember } = this.state
    curMember.curPassword = data.value
    this.setState({ curMember: curMember, passwordError: null })
  }

  handleEditPassword = (e, data) => {
    const { curMember } = this.state
    curMember.password = data.value
    this.setState({ curMember: curMember, passwordError: null })
  }

  handleEditRepeatPassword = (e, data) => {
    const { curMember } = this.state
    curMember.repeatPassword = data.value
    this.setState({ curMember: curMember, passwordError: null })
  }

  handleEditIsAdmin = (e, data) => {
    const { curMember } = this.state
    curMember.isAdmin = data.checked
    this.setState({ curMember: curMember })
  }

  handleCancel = (e) => {
    e.preventDefault()
    this.close()
  }

  close = () => {
    this.props.history.goBack()
  }

  handleSubmit = (e) => {
    e.preventDefault()
    const { type } = this.props
    const { curMember, file, cropData } = this.state

    let avatarData

    if (cropData) {
      avatarData = {
        cropX: cropData.x,
        cropY: cropData.y,
        cropSize: cropData.width,
        file: file
      }
    }

    if (type === 'edit') {
      let updateMemberChange =
        {
          uid: curMember.uid,
          isAdmin: curMember.isAdmin,
          userName: curMember.userName,
          fullName: curMember.fullName,
          email: curMember.email
        }

      Object.assign(updateMemberChange, { avatarData })
      console.log('updateMemberChange', updateMemberChange)

      this.setState({submitting: true})
      this.props.updateMember(updateMemberChange)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.updateMember.hasErrors) {
        if (data.updateMember.genericError) {
          this.setState({showError: true, errorMessage: data.updateMember.genericError})
        }
        if (data.updateMember.updateMemberChangeErrors.userName) {
          this.setState({userNameError: data.updateMember.updateMemberChangeErrors.userName})
        }
        if (data.updateMember.updateMemberChangeErrors.fullName) {
          this.setState({fullNameError: data.updateMember.updateMemberChangeErrors.fullName})
        }
        if (data.updateMember.updateMemberChangeErrors.email) {
          this.setState({emailError: data.updateMember.updateMemberChangeErrors.email})
        }
        if (data.updateMember.updateMemberChangeErrors.avatarData) {
          this.setState({avatarError: data.updateMember.updateMemberChangeErrors.avatarData})
        }
      } else {
        this.setState({profileUpdated: true})
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
    }

    if (type === 'new') {
      let createMemberChange =
        {
          isAdmin: curMember.isAdmin,
          userName: curMember.userName,
          fullName: curMember.fullName,
          email: curMember.email,
          password: curMember.password
        }

      Object.assign(createMemberChange, { avatarData })
      console.log('createMemberChange', createMemberChange)

      this.setState({submitting: true})
      this.props.createMember(createMemberChange)
    .then(({ data }) => {
      this.setState({submitting: false})
      console.log('got data', data)
      if (data.createMember.hasErrors) {
        if (data.createMember.genericError) {
          this.setState({showError: true, errorMessage: data.createMember.genericError})
        }
        if (data.createMember.createMemberChangeErrors.userName) {
          this.setState({userNameError: data.createMember.createMemberChangeErrors.userName})
        }
        if (data.createMember.createMemberChangeErrors.fullName) {
          this.setState({fullNameError: data.createMember.createMemberChangeErrors.fullName})
        }
        if (data.createMember.createMemberChangeErrors.email) {
          this.setState({emailError: data.createMember.createMemberChangeErrors.email})
        }
        if (data.createMember.createMemberChangeErrors.password) {
          this.setState({passwordError: data.createMember.createMemberChangeErrors.password})
        }
        if (data.updateMember.updateMemberChangeErrors.avatarData) {
          this.setState({avatarError: data.updateMember.updateMemberChangeErrors.avatarData})
        }
      } else {
        this.setState({profileCreated: true})
      }
    }).catch((error) => {
      this.setState({submitting: false})
      console.log('there was an error sending the query', error)
    })
    }
  }

  handleImageChange = (e) => {
    e.preventDefault()

    let reader = new window.FileReader()
    let file = e.target.files[0]

    // reset value e.target

    reader.onloadend = () => {
      console.log('reader onloadend')

      if (reader.error) {
        return
      }

      const crop = {
        aspect: 1,
        x: 10,
        y: 10,
        width: 80,
        height: 80
      }

      this.setState({
        crop: crop,
        cropping: true,
        file: file,
        imagePreviewUrl: reader.result
      })
    }

    reader.readAsDataURL(file)
  }

  handleCropCompleted = (crop) => {
    console.log('crop', crop)
    this.setState({ crop })
  }

  handleCropImageDone = (e) => {
    const { imagePreviewUrl, crop } = this.state

// no crop selection
    if (!crop.width) {
      return this.handleCropImageCancel(e)
    }
    e.preventDefault()

    var image = new window.Image()
    image.src = imagePreviewUrl

    var imageWidth = image.naturalWidth
    var imageHeight = image.naturalHeight

    let cropData = {
      x: Math.floor((crop.x / 100) * imageWidth),
      y: Math.floor((crop.y / 100) * imageHeight),
      width: Math.floor((crop.width / 100) * imageWidth),
      height: Math.floor((crop.height / 100) * imageHeight)
    }

    let croppedAvatar = this.cropImage(imagePreviewUrl, crop)
    this.setState({ avatarUpdated: true, imagePreviewUrl: null, cropping: false, croppedAvatar: croppedAvatar, cropData: cropData, crop: { aspect: 1 } })
  }

  handleCropImageCancel = (e) => {
    e.preventDefault()

    this.setState({ avatarUpdated: false, imagePreviewUrl: null, cropping: false, crop: { aspect: 1 } })
  }

  loadImage = (src, callback) => {
    var image = new window.Image()
    image.src = src
  }

  cropImage = (imgSrc, crop) => {
    var image = new window.Image()
    image.src = imgSrc

    var imageWidth = image.naturalWidth
    var imageHeight = image.naturalHeight

    var cropX = (crop.x / 100) * imageWidth
    var cropY = (crop.y / 100) * imageHeight

    var cropWidth = (crop.width / 100) * imageWidth
    var cropHeight = (crop.height / 100) * imageHeight

    var canvas = document.createElement('canvas')
    canvas.width = cropWidth
    canvas.height = cropHeight
    var ctx = canvas.getContext('2d')

    ctx.drawImage(image, cropX, cropY, cropWidth, cropHeight, 0, 0, cropWidth, cropHeight)

    // png to keep image alpha
    return canvas.toDataURL('image/png')
  }

  render () {
    const { viewerQuery, memberQuery } = this.props

    if (Util.isQueriesError(viewerQuery, memberQuery)) {
      return null
    }

    if (Util.isQueriesLoading(viewerQuery, memberQuery)) {
      return null
    }

    const viewer = viewerQuery.viewer
    const { mode, type } = this.props
    const { submitting, curMember, imagePreviewUrl, cropping, crop, avatarUpdated, croppedAvatar, showError, errorMessage, userNameError, fullNameError, emailError, passwordError, profileCreated, profileUpdated } = this.state

    let title
    let submitText
    let edit = type === 'edit'
    if (type === 'edit') {
      title = 'Edit Member'
      submitText = 'Update Profile'
    }
    if (type === 'new') {
      title = 'New Member'
      submitText = 'Create Member'
    }

    let disabled = false

    if ((edit) || (type === 'new')) {
      if (curMember.password !== curMember.repeatPassword) disabled = true
    }

    if (profileCreated) {
      return (
        <Message positive>
          <span>Profile successfully created</span>
        </Message>
      )
    }
    if (profileUpdated) {
      return (
        <Message positive>
          <span>Profile successfully updated</span>
        </Message>
      )
    }

    return (
      <div>
        { cropping &&
          <Modal size='small' open={cropping} onClose={this.handleCropImageCancel} closeOnRootNodeClick={false} closeIcon='close'>
            <Modal.Header>Crop your profile picture</Modal.Header>
            <Modal.Content>
              <ReactCrop src={imagePreviewUrl} crop={crop} onComplete={this.handleCropCompleted} />
            </Modal.Content>
            <Modal.Actions>
              <Button color='green' onClick={this.handleCropImageDone}>Done</Button>
              <Button color='green' onClick={this.handleCropImageCancel}>Cancel</Button>
            </Modal.Actions>
          </Modal>
    }
        <div>
          { mode !== 'self' &&
            <h2>{title}</h2>
        }
          <Form className='clearfix'>
            <Form.Field>
              <label>Profile picture</label>
              { edit && !avatarUpdated &&
                <Avatar uid={curMember.uid} size={200} shape='rounded' />
              }
              { avatarUpdated &&
                <Image width='200px' height='200px' shape='rounded' src={croppedAvatar} />
              }
            </Form.Field>
            <Form.Field inline>
              <Button as='label' className='button' htmlFor='fileupload'>Upload a new picture</Button>
              { /* Fast and ugly way to generate a new input element every time a page is rendered to fix chrome not emitting onChange events when opening the same previously opened file */ }
              <input key={Math.random()} id='fileupload' type='file' style={{display: 'none'}} onChange={(e) => this.handleImageChange(e)} />
            </Form.Field>
            <Form.Field>
              <label>UserName</label>
              <Input name='userName' placeholder='UserName' value={curMember.userName} onChange={this.handleEditUserName} />
              {userNameError && <Label basic color='red' pointing>{userNameError}</Label> }
            </Form.Field>
            <Form.Field>
              <label>FullName</label>
              <Input name='fullName' placeholder='FullName' value={curMember.fullName} onChange={this.handleEditFullName} />
              {fullNameError && <Label basic color='red' pointing>{fullNameError}</Label> }
            </Form.Field>
            <Form.Field>
              <label>Email</label>
              <Input name='email' placeholder='Email' value={curMember.email} onChange={this.handleEditEmail} />
              {emailError && <Label basic color='red' pointing>{emailError}</Label> }
            </Form.Field>
            { type === 'new' &&
            <Form.Field>
              <label>Password</label>
              <Input name='password' placeholder='Password' type='password' value={curMember.password} onChange={this.handleEditPassword} />
              {passwordError && <Label basic color='red' pointing>{passwordError}</Label> }
            </Form.Field>
            }
            { type === 'new' &&
            <Form.Field>
              <label>Repeat Password</label>
              <Input name='repeat password' placeholder='Repeat Password' type='password' value={curMember.repeatPassword} onChange={this.handleEditRepeatPassword} />
            </Form.Field>
            }
            { viewer.member.isAdmin &&
            <Form.Field control={Checkbox} label='Admin' checked={curMember.isAdmin} onChange={this.handleEditIsAdmin} />
            }
            <Button floated='right' color='green' disabled={disabled || submitting} onClick={this.handleSubmit}>{submitText}</Button>
            { !mode === 'self' &&
            <Button floated='right' disabled={submitting} onClick={this.handleCancel}>Cancel</Button>
            }
            <span style={{clear: 'both'}} />
          </Form>
          <Message negative hidden={!showError} onDismiss={this.handleErrorMessageDismiss}>
            <p>{errorMessage}</p>
          </Message>
        </div>
      </div>
    )
  }
}

EditMember.propTypes = {
  mode: PropTypes.string.isRequired,
  type: PropTypes.string.isRequired
}

const createMember = gql`
  mutation createMember($createMemberChange: CreateMemberChange!) {
    createMember(createMemberChange: $createMemberChange) {
      hasErrors
      genericError
      createMemberChangeErrors {
        userName
        fullName
        email
        password
      }
      member {
        uid
        isAdmin
        userName
        fullName
        email
      }
    }
  }
`
const updateMember = gql`
  mutation updateMember($updateMemberChange: UpdateMemberChange!) {
    updateMember(updateMemberChange: $updateMemberChange) {
      hasErrors
      genericError
      updateMemberChangeErrors {
        userName
        fullName
        email
      }
      member {
        uid
        isAdmin
        userName
        fullName
        email
      }
    }
  }
`

const setMemberPassword = gql`
  mutation setMemberPassword($memberUID: ID!, $curPassword: String!, $newPassword: String!) {
    setMemberPassword(memberUID: $memberUID, curPassword: $curPassword, newPassword: $newPassword) {
      hasErrors
      genericError
    }
  }
`

const MemberQuery = gql`
  query memberQuery($uid: ID!) {
    member(uid: $uid) {
      uid
      isAdmin
      userName
      fullName
      email
    }
  }
`

const ViewerQuery = gql`
  query viewerQuery {
    viewer {
      member {
        uid
        isAdmin
        userName
        fullName
        email
      }
    }
  }
`

export default withRouter(compose(
graphql(createMember, {
  name: 'createMember',
  props: ({ createMember }) => ({
    createMember: (createMemberChange) => createMember({ variables: { createMemberChange }, refetchQueries: ['memberQuery'] })
  })
}),
graphql(updateMember, {
  name: 'updateMember',
  props: ({ updateMember }) => ({
    updateMember: (updateMemberChange) => updateMember({ variables: { updateMemberChange }, refetchQueries: ['memberQuery'] })
  })
}),
graphql(setMemberPassword, {
  name: 'setMemberPassword',
  props: ({ setMemberPassword }) => ({
    setMemberPassword: (memberUID, curPassword, newPassword) => setMemberPassword({ variables: { memberUID, curPassword, newPassword }, refetchQueries: ['memberQuery'] })
  })
}),
graphql(MemberQuery, {
  name: 'memberQuery',
  skip: (props) => props.type === 'new' || props.mode === 'self',
  options: props => ({
    variables: {
      uid: props.match.params.memberUID
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
)(withError(EditMember)))
