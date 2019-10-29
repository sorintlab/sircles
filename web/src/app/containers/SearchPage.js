import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Container, Label, Segment, Message, Popup } from 'semantic-ui-react'

import Util from '../modules/Util'
import Avatar from '../components/Avatar'

class SearchPage extends React.Component {

  componentWillReceiveProps (nextProps) {
    console.log(this.props.location)
    console.log(nextProps.location)

    // if same path but new location (user asked for same word) refetch the searchQuery
    if (this.props.location !== nextProps.location && this.props.location.pathname === nextProps.location.pathname) {
      if (this.props.searchQuery) {
        console.log('refetching')
        this.props.searchQuery.refetch()
      }
    }
  }

  render () {
    const { searchQuery } = this.props

    console.log(this.props)

    if (Util.isQueriesError(searchQuery)) {
      return (
        <Container>
          <Message negative>
            <Message.Header>There was an error executing the search</Message.Header>
          </Message>
        </Container>
      )
    }

    if (Util.isQueriesLoading(searchQuery)) {
      return null
    }

    const searchResult = searchQuery.search
    const result = JSON.parse(searchResult.result)

    console.log('result', result)

    return (
      <Container>
        <p>There were {searchResult.totalHits} results</p>

        { result.hits.map(hit => {
          if (hit.fields.Type === 'role') {
            console.log('roletype', hit.fields.RoleType)
            const roleLink = `/role/${hit.id}`
            return (
              <Segment>
                <Link to={roleLink}>
                  {hit.fields.Name}
                </Link>
                {hit.fields.RoleType === 'circle' && <Label className='labelright' color='blue' horizontal basic size='tiny'>Circle</Label> }
                {hit.fields.RoleType === 'normal' && <Label className='labelright' color='teal' horizontal basic size='tiny'>Role</Label> }
              </Segment>
            )
          }
          if (hit.fields.Type === 'member') {
            const memberLink = `/member/${hit.id}`
            return (
              <Segment>
                <Link to={memberLink}>
                  <Avatar uid={hit.id} size={30} inline spaced shape='rounded' />
                  <Popup content={hit.fields.FullName} trigger={
                    <span>
                      {hit.fields.UserName}
                    </span>
                  } />
                </Link>
                <Label className='labelright' color='green' horizontal basic size='tiny'>Member</Label>
              </Segment>
            )
          }
        })

        }
      </Container>
    )
  }
}

SearchPage.propTypes = {
  searchQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired
  }).isRequired
}

const SearchPageQuery = gql`
  query searchPageQuery($query: String!) {
    search(query: $query) {
      totalHits
      hits
      result
    }
  }
`

export default compose(
graphql(SearchPageQuery, {
  name: 'searchQuery',
  options: props => ({
    variables: {
      query: props.match.params.query
    },
    fetchPolicy: 'network-only'
  })
}),
)(SearchPage)
