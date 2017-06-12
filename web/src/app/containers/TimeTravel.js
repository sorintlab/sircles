import React from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { withRouter } from 'react-router-dom'
import { Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'

class TimeTravel extends React.Component {
  componentWillReceiveProps (nextProps) {
    const { timeLineFromTimeQuery } = nextProps

    if (timeLineFromTimeQuery.error) {
      this.props.appError.setError(true)
      return
    }

    if (timeLineFromTimeQuery.loading) {
      return
    }

    const timeLineID = timeLineFromTimeQuery.timeLines.edges[0].timeLine.id
    this.props.history.push(`/timeline/${timeLineID}`)
  }

  render () {
    console.log('props', this.props)

    const { timeLineFromTimeQuery } = this.props

    if (!timeLineFromTimeQuery) {
      return null
    }

    if (timeLineFromTimeQuery.error) {
      return null
    }

    if (timeLineFromTimeQuery.loading) {
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

export default withRouter(compose(
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
