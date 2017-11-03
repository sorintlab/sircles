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
    const { timeLineQuery, timeLineAfter, timeLineBefore } = nextProps

    if (Util.isQueriesError(timeLineQuery, timeLineAfter, timeLineBefore)) {
      this.props.appError.setError(true)
      return
    }
  }

  updateTimeLineUrl = (inc) => {
    const { timeLineAfter, timeLineBefore } = this.props

    let timeLineNumber
    if (inc) {
      timeLineNumber = timeLineAfter.timeLines.edges[0].timeLine.id
    } else {
      timeLineNumber = timeLineBefore.timeLines.edges[0].timeLine.id
    }

    let path = this.props.location.pathname
    path = path.replace(/\/timeline\/\d+/, `/timeline/${timeLineNumber}`)
    this.props.history.push(path)
  }

  render () {
    console.log('props', this.props)

    const { timeLineQuery, timeLineAfter, timeLineBefore } = this.props

    if (!timeLineQuery) {
      return null
    }

    if (Util.isQueriesError(timeLineQuery, timeLineAfter, timeLineBefore)) {
      return null
    }

    if (Util.isQueriesLoading(timeLineQuery, timeLineAfter, timeLineBefore)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    return (
      <Message warning>
        <Message.Header>Time travelling your organization at {moment(timeLineQuery.timeLine.time).format('LLLL')}</Message.Header>
        <Message.Content>
          <Button disabled={timeLineBefore.timeLines.edges[0] == null} onClick={() => { this.updateTimeLineUrl(false) }}>Previous Change</Button>
          <Button disabled={timeLineAfter.timeLines.edges[0] == null} onClick={() => { this.updateTimeLineUrl(true) }}>Next Change</Button>
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

const timeLineAfter = gql`
  query timeLineAfterTime($fromID: String) {
    timeLines(first: 1, aggregateType: "rolestree", fromID: $fromID) {
      edges {
        timeLine {
          id
          time
        }
      }
    }
  }
`

const timeLineBefore = gql`
  query timeLineBeforeTime($fromID: String) {
    timeLines(last: 1, aggregateType: "rolestree", fromID: $fromID) {
      edges {
        timeLine {
          id
          time
        }
      }
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
graphql(timeLineAfter, {
  name: 'timeLineAfter',
  skip: (props) => !props.match.params.timeLine,
  options: props => ({
    variables: {
      fromID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(timeLineBefore, {
  name: 'timeLineBefore',
  skip: (props) => !props.match.params.timeLine,
  options: props => ({
    variables: {
      fromID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
}),
)(withError(TimeTravelMessage))
