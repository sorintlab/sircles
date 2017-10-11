import React from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { withRouter } from 'react-router-dom'
import { Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Util from '../modules/Util'

class TimeTravel extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { timeLineFromTimeQuery, curTimeLineQuery } = nextProps

    if (Util.isQueriesError(timeLineFromTimeQuery, curTimeLineQuery)) {
      this.props.appError.setError(true)
      return
    }

    if (Util.isQueriesLoading(timeLineFromTimeQuery, curTimeLineQuery)) {
      return
    }

    let timeLineID = curTimeLineQuery.timeLine.id
    if (timeLineFromTimeQuery.timeLines.edges[0]) {
      timeLineID = timeLineFromTimeQuery.timeLines.edges[0].timeLine.id
    }

    this.props.history.push(`/timeline/${timeLineID}`)
  }

  render () {
    console.log('props', this.props)

    const { timeLineFromTimeQuery } = this.props

    if (!timeLineFromTimeQuery) {
      return null
    }

    if (Util.isQueriesError(timeLineFromTimeQuery, curTimeLineQuery)) {
      return null
    }

    if (Util.isQueriesLoading(timeLineFromTimeQuery, curTimeLineQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    return null
  }
}

const timeLineFromTimeQuery = gql`
  query timeLineFromTime($fromTime: Time) {
    timeLines(first: 1, fromTime: $fromTime) {
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
graphql(timeLineFromTimeQuery, {
  name: 'timeLineFromTimeQuery',
  options: props => ({
    variables: {
      fromTime: props.day
    },
    fetchPolicy: 'network-only'
  })
}),
)(withError(TimeTravel)))
