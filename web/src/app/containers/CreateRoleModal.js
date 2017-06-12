import React, { PropTypes } from 'react'
import { Modal } from 'semantic-ui-react'

import CreateRole from './CreateRole'

class CreateRoleModal extends React.Component {

  onClose = () => {
    this.props.onClose()
  }

  render () {
    return (
      <Modal open={this.props.open} onClose={this.props.onClose} closeIcon='close'>
        <Modal.Header>Add new Role</Modal.Header>
        <Modal.Content>
          <CreateRole parentRoleUID={this.props.parentRoleUID} onClose={this.onClose} />
        </Modal.Content>
      </Modal>
    )
  }
}

CreateRoleModal.propTypes = {
  parentRoleUID: PropTypes.string.isRequired
}

export default CreateRoleModal
