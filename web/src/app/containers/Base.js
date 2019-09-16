import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import { Route, Link, Switch } from 'react-router-dom'
import { Container, Menu, Dropdown, Message, Form, Input, Button, Image } from 'semantic-ui-react'

import config from 'config'
import Auth from '../modules/Auth'
import Util from '../modules/Util'
import ViewerQuery from '../graphql/ViewerQuery'
import Avatar from '../components/Avatar'

import UserMenu from './UserMenu'
import OrgChart from './OrgChart'
import TimeTravelMessage from './TimeTravelMessage'
import LoginPage from './LoginPage'
import OIDCLoginPage from './OIDCLoginPage'
import OIDCCallbackPage from './OIDCCallbackPage'
import RolePage from './RolePage'
import Member from './Member'
import MemberTensions from './MemberTensions'
import Tension from './Tension'
import EditTension from './EditTension'
import SearchPage from './SearchPage'
import Settings from './Settings'

import sirclesLogo from '../../www/img/sircles.png'

class Base extends React.Component {
  getChildContext () {
    return {appError: this.props.appError}
  }

  componentWillMount () {
    this.resetComponent()
    console.log('user authenticated', Auth.isUserAuthenticated())
    console.log('this.props.location', this.props.location)

    if (!Auth.isUserAuthenticated()) {
      if (this.props.location.pathname !== '/login' && this.props.location.pathname !== '/login/callback') {
        this.props.history.push('/login')
      }
    }

    console.log(this.props.appError)
    this.props.appError.listen((error) => { console.log('listen, error:', error); this.setState({error: error}) })
  }

  resetComponent = () => this.setState({ searchString: '' })

  componentWillReceiveProps (nextProps) {
    const { viewerQuery } = nextProps

    console.log('user authenticated', Auth.isUserAuthenticated())
    console.log('nextProps.location', nextProps.location)

    if (!Auth.isUserAuthenticated()) {
      if (nextProps.location.pathname !== '/login' && nextProps.location.pathname !== '/login/callback') {
        this.props.history.push('/login')
      }
    }

    if (Util.isQueriesError(viewerQuery)) {
      this.props.appError.setError(true)
      return
    }

    this.props.appError.setError(false)

    // reset search string when moving out from search page
    if (this.props.location.pathname !== nextProps.location.pathname) {
      if (!nextProps.location.pathname.startsWith('/search/')) {
        this.setState({searchString: ''})
      }
    }
  }

  logout = () => {
    Auth.deauthenticateUser()
    this.props.client.resetStore()
    this.props.history.push('/login')
  }

  handleSearchChange = (e, data) => {
    e.preventDefault()
    this.setState({searchString: data.value})
  }

  doSearch = (e, data) => {
    e.preventDefault()
    const { searchString } = this.state
    this.props.history.push(`/search/${searchString}`)
  }

  render () {
    const { viewerQuery } = this.props
    const { error, searchString } = this.state

    if (error) {
    /* TODO(sgotti) we are calling windows.location.reload because looks like apollo client (<= 1.0.0-rc.2) when calling the query refetch() returns error also if the query succeeded */
      return (
        <Container>
          <Message negative>
            <Message.Header>Error contacting the server</Message.Header>
            <Button onClick={() => { window.location.reload() }}>Reload</Button>
          </Message>
        </Container>
      )
    }

    let viewer
    if (viewerQuery) {
      if (viewerQuery.error) {
        return null
      }

      if (Util.isQueriesLoading(viewerQuery)) {
        return null
      }

      viewer = viewerQuery.viewer
    }

    return (
      <div>
        <Menu inverted attached>
          <Menu.Item as={Link} to='/'>
            <Image size='tiny' src={sirclesLogo} />
          </Menu.Item>
      
          {viewer && (
            <Menu.Item as={Form} onSubmit={this.doSearch}>
              <Input icon='search' placeholder='Search...' value={searchString} onChange={this.handleSearchChange} />
            </Menu.Item>
        )}
          {viewer ? (
            <Menu.Menu position='right'>
              <Dropdown className='item' trigger={
                <span>
                  <Avatar uid={viewer.member.uid} size={30} inline spaced shape='rounded' />
                  {viewer.member.userName}
                </span>
                }>
                <Dropdown.Menu>
                  <Dropdown.Item as={Link} to='/settings/profile'>Settings</Dropdown.Item>
                </Dropdown.Menu>
              </Dropdown>
              <Menu.Item onClick={this.logout} link>Log out</Menu.Item>
            </Menu.Menu>
          ) : (
            <Menu.Menu position='right'>
              <Menu.Item as={Link} to='/login'>Log in</Menu.Item>
            </Menu.Menu>
          )}
        </Menu>

        <Route path='/timeline/:timeLine' component={TimeTravelMessage} />

        {viewer &&
        <Switch>
          <Route path='/timeLine/:timeLine' render={(props) => <UserMenu viewer={viewer} {...props} />} />
          <Route render={(props) => <UserMenu viewer={viewer} {...props} />} />
        </Switch>
        }

        <Switch>
          <Route exact path='/' component={OrgChart} />
          <Route exact path='/timeline/:timeLine' component={OrgChart} />
          <Route path='/orgchart/:node?' component={OrgChart} />
          <Route path='/timeline/:timeLine/orgchart/:node?' component={OrgChart} />

          { config.authType === 'oidc'
            ? <Route exact path='/login' component={OIDCLoginPage} />
            : <Route exact path='/login' component={LoginPage} />
          }

          <Route exact path='/login/callback' component={OIDCCallbackPage} />

          <Route path='/settings' component={Settings} />

          <Route path='/role/:roleUID' component={RolePage} />
          <Route path='/timeline/:timeLine/role/:roleUID' component={RolePage} />

          <Route path='/member/:memberUID' component={Member} />
          <Route path='/timeline/:timeLine/member/:memberUID' component={Member} />

          <Route path='/tensions' component={MemberTensions} />
          <Route exact path='/tension/new' render={(props) => <EditTension type='new' {...props} />} />
          <Route exact path='/tension/:tensionUID/edit' render={(props) => <EditTension type='edit' {...props} />} />
          <Route exact path='/tension/:tensionUID' component={Tension} />

          <Route path='/search/:query' component={SearchPage} />
        </Switch>
      </div>
    )
  }
}

Base.propTypes = {
  viewerQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired,
    viewer: PropTypes.object
  })
}

Base.childContextTypes = {
  appError: PropTypes.object
}

export default compose(
graphql(ViewerQuery, {
  name: 'viewerQuery',
  skip: () => !Auth.isUserAuthenticated(),
  options: () => ({
    fetchPolicy: 'network-only'
  })
})
)(Base)
