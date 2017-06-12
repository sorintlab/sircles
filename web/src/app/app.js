import React from 'react'
import ReactDom from 'react-dom'
import injectTapEventPlugin from 'react-tap-event-plugin'
import { Router, Route } from 'react-router-dom'
import createBrowserHistory from 'history/createBrowserHistory'

import ApolloClient from 'apollo-client'
import { ApolloProvider } from 'react-apollo'
import {createNetworkInterface} from 'apollo-upload-client'

import Error from './modules/Error'
import Auth from './modules/Auth'
import config from 'config'

import Base from './containers/Base'

import '../../semantic/dist/semantic.min.css'
import 'react-image-crop/dist/ReactCrop.css'
import '../www/css/style.css'

import '../www/index.html'

injectTapEventPlugin()

const browserHistory = createBrowserHistory()

const networkInterface = createNetworkInterface({ uri: config.apiBaseUrl + '/graphql' })

const appError = new Error()

networkInterface.use([{
  applyMiddleware (req, next) {
    if (!req.options.headers) {
      req.options.headers = {}  // Create the header object if needed.
    }

    // get the authentication token from local storage if it exists
    const token = Auth.getToken()
    // TODO(sgotti) detect expiring token ans issue a refresh request
    req.options.headers.authorization = token ? `Bearer ${token}` : null
    next()
  }
}])

networkInterface.useAfter([{
  applyAfterware ({ response }, next) {
    console.log('response', response)
    if (response.status === 401) {
      if (Auth.isUserAuthenticated()) {
        Auth.deauthenticateUser()
      }
    }
    next()
  }
}])

const client = new ApolloClient({
  networkInterface
  // the server returns global ids
  // TODO(sgotti) temporary disabled due to possible apollo bug returning an undefined object
  // dataIdFromObject: o => o.id
})

ReactDom.render((
  <ApolloProvider client={client}>
    <Router history={browserHistory}>
      <Route render={(props) => <Base appError={appError} {...props} />} />
    </Router>
  </ApolloProvider>
), document.getElementById('react-app'))
