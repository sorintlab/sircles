import React, { PropTypes } from 'react'
import { Image } from 'semantic-ui-react'

import Util from '../modules/Util'

const Avatar = (props) => {
  const { uid, size, ...passThroughProps } = props
  if (size) {
    return <Image width={`${size}px`} height={`${size}px`} src={Util.avatarUrl(uid, size)} {...passThroughProps} />
  }
  return <Image src={Util.avatarUrl(uid, size)} {...passThroughProps} />
}

Avatar.propTypes = {
  uid: PropTypes.string.isRequired,
  size: PropTypes.number
}

export default Avatar
