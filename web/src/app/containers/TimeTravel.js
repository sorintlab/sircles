import React from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { withRouter } from 'react-router-dom'
import { Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class TimeTravel extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery } = nextProps

    if (Util.isQueriesError(timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery)) {
      this.props.appError.setError(true)
      return
    }

    if (Util.isQueriesLoading(timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery)) {
      return
    }

    let timeLineID = curTimeLineQuery.timeLine.id
    if (timeLineAfterTimeQuery.timeLines.edges.length > 0) {
      if (timeLineAfterTimeQuery.timeLines.edges[0]) {
        timeLineID = timeLineAfterTimeQuery.timeLines.edges[0].timeLine.id
      }
    } else if (timeLineBeforeTimeQuery.timeLines.edges.length > 0) {
      if (timeLineBeforeTimeQuery.timeLines.edges[0]) {
        timeLineID = timeLineBeforeTimeQuery.timeLines.edges[0].timeLine.id
      }
    }

    this.props.history.push(`/timeline/${timeLineID}`)
  }

  render () {
    console.log('props', this.props)

    const { timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery } = this.props

    if (Util.isQueriesError(timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery)) {
      return null
    }

    if (Util.isQueriesLoading(timeLineAfterTimeQuery, timeLineBeforeTimeQuery, curTimeLineQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    return null
  }
}

const timeLineAfterTimeQuery = gql`
  query timeLineAfterTime($fromTime: Time) {
    timeLines(first: 1, aggregateType: "rolestree", fromTime: $fromTime) {
      edges {
        timeLine {
          id
        }
      }
    }
  }
`

const timeLineBeforeTimeQuery = gql`
  query timeLineBeforeTime($fromTime: Time) {
    timeLines(last: 1, aggregateType: "rolestree", fromTime: $fromTime) {
      edges {
        timeLine {
          id
        }
      }
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

export default withRouter(compose(
graphql(curTimeLineQuery, {
  name: 'curTimeLineQuery',
  options: props => ({
    fetchPolicy: 'network-only'
  })
}),
graphql(timeLineAfterTimeQuery, {
  name: 'timeLineAfterTimeQuery',
  options: props => ({
    variables: {
      fromTime: props.day
    },
    fetchPolicy: 'network-only'
  })
}),
graphql(timeLineBeforeTimeQuery, {
  name: 'timeLineBeforeTimeQuery',
  options: props => ({
    variables: {
      fromTime: props.day
    },
    fetchPolicy: 'network-only'
  })
}),
)(withError(TimeTravel)))
