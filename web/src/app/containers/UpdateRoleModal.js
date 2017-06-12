import React, { PropTypes } from 'react'
import { Modal } from 'semantic-ui-react'

import UpdateRole from './UpdateRole'

class UpdateRoleModal extends React.Component {

  onClose = () => {
    this.props.onClose()
  }

  render () {
    return (
      <Modal open={this.props.open} onClose={this.props.onClose} closeIcon='close'>
        <Modal.Header>Edit Role {this.props.roleName}</Modal.Header>
        <Modal.Content>
          <UpdateRole parentRoleUID={this.props.parentRoleUID} roleUID={this.props.roleUID} onClose={this.onClose} />
        </Modal.Content>
      </Modal>
    )
  }
}

UpdateRoleModal.propTypes = {
  parentRoleUID: PropTypes.string.isRequired,
  roleUID: PropTypes.string.isRequired,
  roleName: PropTypes.string.isRequired
}

export default UpdateRoleModal
