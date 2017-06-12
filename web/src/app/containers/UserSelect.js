import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import React from 'react'
import { Dropdown } from 'semantic-ui-react'

const defaultFetchSize = 25

const typeToSearchMessage = 'Type to search for a member'
const noMembersFoundMessages = 'No members found'

class UserSelect extends React.Component {
  componentWillMount () {
    console.log('userselect will mount')
    this.resetComponent()
  }

  resetComponent = () => {
    this.setState({ value: '', searchString: '', searchError: false })
    this.callOnValueChange('')
  }

  callOnValueChange = (value) => {
    const { onValueChange } = this.props
    if (onValueChange) onValueChange(value)
  }

  handleChange = (e, data) => {
    const value = data.value
    this.setState({value: value})
    this.callOnValueChange(value)
  }

  handleOpen = (e, data) => {
    this.setState({ searchString: '', value: '' })
    this.callOnValueChange('')
  }

  handleSearchChange = (e, value) => {
    this.setState({ searchString: value })

    if (value.length > 0) {
      this.props.membersQuery.update(value).then(() => {
        this.setState({ searchError: false })
      }).catch((error) => {
        console.log('there was an error sending the query', error)
        this.setState({ searchError: true })
      })
    }
  }

  render () {
    const { value } = this.state
    const { ...rest } = this.props

    const { searchString, searchError } = this.state

    let isLoading = this.props.membersQuery.loading

    let options = []
    if (this.props.membersQuery.loading !== true && !this.props.membersQuery.error) {
      if (searchString.length > 0) {
        options = this.props.membersQuery.members.map(member => { return {text: `${member.fullName} (${member.userName})`, value: member.uid} })
      }
    }

    let noResultsMessage = typeToSearchMessage
    if (searchString.length > 0) {
      noResultsMessage = noMembersFoundMessages
    }

    return (
      <Dropdown
        fluid
        selection
        search
        selectOnBlur={false}
        loading={isLoading}
        error={searchError}
        noResultsMessage={noResultsMessage}
        options={options}
        value={value}
        placeholder='Enter a username'
        onOpen={this.handleOpen}
        onChange={this.handleChange}
        onSearchChange={this.handleSearchChange}
        {...rest}
      />
    )
  }
}

const MembersQuery = gql`
  query MembersQuery($first: Int, $search: String){
    members(first: $first, search: $search) {
      edges {
        member {
          uid
          userName
          fullName
        }
      }
      hasMoreData
    }
  }
`

export default compose(
graphql(MembersQuery, {
  options: () => ({
    variables: {
      first: defaultFetchSize
    },
    fetchPolicy: 'network-only'
  }),
  props ({ data: { loading, error, refetch, members, fetchMore } }) {
    const membersList = members && members.edges.map((e) => (e.member))
    return {
      membersQuery: {
        loading,
        error,
        members: membersList,
        hasMoreData: members && members.hasMoreData,
        update: (searchString) => {
          return fetchMore({
            query: MembersQuery,
            variables: {
              first: defaultFetchSize,
              search: searchString
            },
            updateQuery: (previousResult, { fetchMoreResult }) => {
              const newEdges = fetchMoreResult.members.edges
              return {
                members: {
                  edges: [...newEdges],
                  hasMoreData: fetchMoreResult.members.hasMoreData
                }
              }
            }
          })
        }
      }
    }
  }
})
)(UserSelect)
