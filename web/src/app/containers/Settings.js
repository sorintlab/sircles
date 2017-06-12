import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import { Route, Link, Switch } from 'react-router-dom'
import { Container, Grid, Divider, Menu, Dimmer, Loader } from 'semantic-ui-react'

import { withError } from '../modules/Error'
import Auth from '../modules/Auth'
import Util from '../modules/Util'
import ViewerQuery from '../graphql/ViewerQuery'

import Members from './Members'
import EditMember from './EditMember'
import UpdateMemberPassword from './UpdateMemberPassword'

class Settings extends React.Component {

  componentWillReceiveProps (nextProps) {
    const { viewerQuery } = nextProps

    if (Util.isQueriesError(viewerQuery)) {
      this.props.appError.setError(true)
      return
    }
  }

  render () {
    const { viewerQuery } = this.props

    let viewer
    if (!viewerQuery) return null

    if (Util.isQueriesError(viewerQuery)) {
      return null
    }

    if (Util.isQueriesLoading(viewerQuery)) {
      return (
        <Dimmer active inverted>
          <Loader inverted>Loading</Loader>
        </Dimmer>
      )
    }

    viewer = viewerQuery.viewer

    return (
      <Container>
        <Grid>
          <Grid.Column width={4}>
            <Menu vertical fluid>
              <Menu.Item>
                <Menu.Header content='Personal settings' />
              </Menu.Item>
              <Menu.Item as={Link} to='/settings/profile'>Profile</Menu.Item>
              <Menu.Item as={Link} to='/settings/authentication'>Authentication</Menu.Item>
            </Menu>
            { viewer.member.isAdmin &&
            <Menu vertical fluid>
              <Menu.Item>
                <Menu.Header content='Administrator zone' />
              </Menu.Item>
              <Menu.Item as={Link} to='/settings/admin/members'>Members</Menu.Item>
            </Menu>
          }
          </Grid.Column>

          <Grid.Column width={12}>
            <Switch>
              <Route path='/settings/profile' render={(props) => <EditMember type='edit' mode='self' {...props} />} />
              <Route path='/settings/authentication' render={(props) => <UpdateMemberPassword mode='self' {...props} />} />

              <Route path='/settings/admin/members' component={Members} />
              <Route exact path='/settings/admin/member/new' render={(props) => <EditMember type='new' mode='member' {...props} />} />
              <Route exact path='/settings/admin/member/:memberUID/edit' render={(props) =>
                <div>
                  <EditMember type='edit' mode='member' {...props} />
                  <Divider clearing />
                  <UpdateMemberPassword mode='member' {...props} />
                </div>
                } />
            </Switch>
          </Grid.Column>
        </Grid>
      </Container>
    )
  }
}

Settings.propTypes = {
  children: PropTypes.object,
  viewerQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired,
    viewer: PropTypes.object
  })
}

export default compose(
graphql(ViewerQuery, {
  name: 'viewerQuery',
  skip: () => !Auth.isUserAuthenticated(),
  options: () => ({
    fetchPolicy: 'network-only'
  })
})
)(withError(Settings))
