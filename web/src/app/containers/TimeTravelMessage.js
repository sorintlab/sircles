import React from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Message, Button, Dimmer, Loader } from 'semantic-ui-react'
import moment from 'moment'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class TimeTravelMessage extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { timeLineQuery, curTimeLineQuery } = nextProps

    if (Util.isQueriesError(timeLineQuery, curTimeLineQuery)) {
      this.props.appError.setError(true)
      return
    }
  }

  updateTimeLineUrl = (inc) => {
    let timeLineNumber = parseInt(this.props.match.params.timeLine, 10) + inc
    if (timeLineNumber < 1) {
      timeLineNumber = 1
    }

    let path = this.props.location.pathname
    path = path.replace(/\/timeline\/\d+/, `/timeline/${timeLineNumber}`)
    this.props.history.push(path)
  }

  render () {
    console.log('props', this.props)

    const { timeLineQuery, curTimeLineQuery } = this.props
    const timeLineNumber = parseInt(this.props.match.params.timeLine, 10)

    if (!timeLineQuery) {
      return null
    }

    if (Util.isQueriesError(timeLineQuery, curTimeLineQuery)) {
      return null
    }

    if (Util.isQueriesLoading(timeLineQuery, curTimeLineQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    const curTimeLine = curTimeLineQuery.timeLine.id

    return (
      <Message warning>
        <Message.Header>Time travelling your organization at {moment(timeLineQuery.timeLine.time).format('LLLL')}</Message.Header>
        <Message.Content>
          <Button disabled={timeLineNumber <= 1} onClick={() => { this.updateTimeLineUrl(-1) }}>Previous Change</Button>
          <Button disabled={timeLineNumber >= curTimeLine} onClick={() => { this.updateTimeLineUrl(+1) }}>Next Change</Button>
          <Button as={Link} to='/'>Exit</Button>
        </Message.Content>
      </Message>
    )
  }
}

const timeLineQuery = gql`
  query timeLine($id: TimeLineID) {
    timeLine(id: $id) {
      id
      time
    }
  }
`

const curTimeLineQuery = gql`
  query curTimeLine {
    timeLine {
      id
      time
    }
  }
`

export default compose(
graphql(timeLineQuery, {
  name: 'timeLineQuery',
  skip: (props) => !props.match.params.timeLine,
  options: props => ({
    variables: {
      id: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(curTimeLineQuery, {
  name: 'curTimeLineQuery',
  options: props => ({
    fetchPolicy: 'network-only'
  })
}),
)(withError(TimeTravelMessage))
