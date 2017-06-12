import React, { PropTypes } from 'react'
import { Modal } from 'semantic-ui-react'

import DeleteRole from './DeleteRole'

class DeleteRoleModal extends React.Component {

  onClose = () => {
    this.props.onClose()
  }

  render () {
    return (
      <Modal open={this.props.open} onClose={this.props.onClose} closeIcon='close'>
        <Modal.Header>Delete Role {this.props.roleName}</Modal.Header>
        <Modal.Content>
          <DeleteRole parentRoleUID={this.props.parentRoleUID} roleUID={this.props.roleUID} onClose={this.onClose} />
        </Modal.Content>
      </Modal>
    )
  }
}

DeleteRoleModal.propTypes = {
  parentRoleUID: PropTypes.string.isRequired,
  roleUID: PropTypes.string.isRequired,
  roleName: PropTypes.string.isRequired
}

export default DeleteRoleModal
